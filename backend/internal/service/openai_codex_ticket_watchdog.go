package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/pkg/logger"
	"github.com/tidwall/gjson"
	"go.uber.org/zap"
)

const codexTicketWatchdogExtraKey = "codex_ticket_watchdog"

// Persist only a small reason/timestamp summary, never response bodies or STATE.
type CodexTicketWatchdogStatus struct {
	Enabled         bool       `json:"enabled"`
	TriggerCount    int64      `json:"trigger_count"`
	LastReason      string     `json:"last_reason,omitempty"`
	LastTriggeredAt *time.Time `json:"last_triggered_at,omitempty"`
}

func codexTicketWatchdogStatusOf(account *Account, enabled bool) CodexTicketWatchdogStatus {
	var status CodexTicketWatchdogStatus
	if account != nil {
		if raw, err := json.Marshal(account.Extra[codexTicketWatchdogExtraKey]); err == nil {
			_ = json.Unmarshal(raw, &status)
		}
	}
	if status.LastReason != "model_mismatch" && status.LastReason != "state_312" {
		status = CodexTicketWatchdogStatus{}
	}
	status.Enabled = enabled
	return status
}

// Bound at the exact injection point. Client-supplied headers cannot opt a
// request into the watchdog, and an old response cannot revoke a newer ticket.
type codexTicketReceipt struct {
	accountID        int64
	model            string
	revision         string
	fixedFingerprint string
	stateHash        [32]byte
	capturedAt       time.Time
}

type codexTicketReceiptContextKey struct{}

type codexTicketInvalidationRetry struct {
	receipt codexTicketReceipt
	reason  string
}

type codexTicketRevocationKey struct {
	accountID                         int64
	model, revision, fixedFingerprint string
	stateHash                         [32]byte
	capturedUnixNano                  int64
}

func (r codexTicketReceipt) key() codexTicketRevocationKey {
	return codexTicketRevocationKey{r.accountID, r.model, r.revision, r.fixedFingerprint, r.stateHash, r.capturedAt.UnixNano()}
}

func receiptForCodexTicket(ticket *openAICodexTicket) codexTicketReceipt {
	return codexTicketReceipt{ticket.AccountID, ticket.Model, ticket.ConfigRevision,
		ticket.FixedProxyFingerprint, sha256.Sum256([]byte(ticket.State)), ticket.CapturedAt}
}

func (r codexTicketReceipt) matches(ticket *openAICodexTicket) bool {
	if ticket == nil {
		return false
	}
	other := receiptForCodexTicket(ticket)
	return r.accountID == other.accountID && r.model == other.model && r.revision == other.revision &&
		r.fixedFingerprint == other.fixedFingerprint && r.stateHash == other.stateHash && r.capturedAt.Equal(other.capturedAt)
}

func (s *OpenAIGatewayService) codexTicketRejectedByWatchdog(ticket *openAICodexTicket) bool {
	if s == nil || ticket == nil {
		return false
	}
	_, revoked := s.openaiCodexWatchdogRevoked.Load(receiptForCodexTicket(ticket).key())
	return revoked
}

func (s *OpenAIGatewayService) applyOpenAICodexTicketToRequest(ctx context.Context, account *Account, model string, req *http.Request) error {
	if req == nil {
		return nil
	}
	receipt, err := s.applyOpenAICodexTicketWithReceipt(ctx, account, model, req.Header)
	if err != nil {
		return err
	}
	*req = *req.WithContext(context.WithValue(req.Context(), codexTicketReceiptContextKey{}, receipt))
	return nil
}

func (s *OpenAIGatewayService) observeCodexTicketResponse(req *http.Request, resp *http.Response) {
	if s == nil || req == nil {
		return
	}
	receipt, _ := req.Context().Value(codexTicketReceiptContextKey{}).(*codexTicketReceipt)
	if receipt == nil || resp == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	var once sync.Once
	trigger := func(reason string) {
		once.Do(func() { s.enqueueCodexTicketInvalidation(*receipt, reason) })
	}
	// 312 is an experimental refresh signal, not an asserted upstream revocation protocol.
	if state := strings.TrimSpace(resp.Header.Get(openAICodexTurnStateHeader)); len(state) == 312 && validCodexTicketState(state) {
		trigger("state_312")
	}
	if resp.Body != nil {
		resp.Body = &codexTicketWatchdogBody{ReadCloser: resp.Body, model: receipt.model, trigger: trigger}
	}
}

// Revoke locally at observation time, then persist without delaying downstream
// streaming. Exact receipt identities also tolerate clock skew between instances.
func (s *OpenAIGatewayService) enqueueCodexTicketInvalidation(receipt codexTicketReceipt, reason string) {
	key := receipt.key()
	expires := time.Now().Add(2 * time.Hour)
	if existing, loaded := s.openaiCodexWatchdogRevoked.LoadOrStore(key, expires); loaded {
		if expiry, ok := existing.(time.Time); ok {
			expires = expiry
		}
	}
	// Successful/no-op invalidations stay deduplicated for the receipt lifetime.
	// A failed persistence attempt may retry after one minute, never per response.
	for {
		previous, exists := s.openaiCodexWatchdogPending.Load(key)
		if !exists {
			if _, loaded := s.openaiCodexWatchdogPending.LoadOrStore(key, expires); !loaded {
				break
			}
			continue
		}
		if retryAt, ok := previous.(time.Time); ok && time.Now().Before(retryAt) {
			return
		}
		if s.openaiCodexWatchdogPending.CompareAndSwap(key, previous, expires) {
			break
		}
	}
	s.openaiCodexAccountMu.Lock()
	if s.openaiCodexAccountStopping {
		s.openaiCodexAccountMu.Unlock()
		return
	}
	s.openaiCodexAccountWG.Add(1)
	s.openaiCodexAccountMu.Unlock()
	go func() {
		defer s.openaiCodexAccountWG.Done()
		if !s.invalidateCodexTicketFromResponse(receipt, reason) {
			s.openaiCodexWatchdogPending.Store(key, time.Now().Add(time.Minute))
			s.openaiCodexWatchdogRetry.Store(key, codexTicketInvalidationRetry{receipt, reason})
		} else {
			s.openaiCodexWatchdogRetry.Delete(key)
		}
	}()
}

// Persistence retries run even when harvesting is disabled and there are no
// subsequent business responses. The ticker launches a bounded batch per process.
func (s *OpenAIGatewayService) retryCodexTicketInvalidations(ctx context.Context) {
	remaining := s.openAICodexTicketConfig().MaxConcurrentHarvests
	s.openaiCodexWatchdogRetry.Range(func(key, value any) bool {
		if ctx.Err() != nil || remaining <= 0 {
			return false
		}
		retry, ok := value.(codexTicketInvalidationRetry)
		if !ok {
			return true
		}
		if raw, pending := s.openaiCodexWatchdogPending.Load(key); pending {
			if after, ok := raw.(time.Time); ok && time.Now().Before(after) {
				return true
			}
		}
		remaining--
		s.enqueueCodexTicketInvalidation(retry.receipt, retry.reason)
		return true
	})
}

func (s *OpenAIGatewayService) invalidateCodexTicketFromResponse(receipt codexTicketReceipt, reason string) bool {
	if reason != "state_312" && reason != "model_mismatch" {
		return true
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	invalidated := false
	err := s.mutateCodexTicket(ctx, receipt.accountID, func(account *Account) (map[string]any, error) {
		if !isOpenAICodexTicketAccount(account) {
			return nil, nil
		}
		ac := codexAccountTicketConfigOf(account)
		current := parseOpenAICodexTicketFromAny(account.ID, receipt.model, account.Extra[openAICodexTicketExtraKey(receipt.model)])
		if !ac.Enabled || ac.Revision != receipt.revision || codexTicketFixedProxyFingerprint(account) != receipt.fixedFingerprint || !receipt.matches(current) {
			return nil, nil
		}
		status := codexTicketWatchdogStatusOf(account, true)
		status.TriggerCount++
		status.LastReason = reason
		now := time.Now()
		status.LastTriggeredAt = &now
		invalidated = true
		return map[string]any{openAICodexTicketExtraKey(receipt.model): nil, codexTicketWatchdogExtraKey: status}, nil
	})
	if err != nil {
		logger.L().Warn("codex ticket invalidation persistence failed", zap.Int64("account_id", receipt.accountID))
	}
	if invalidated || err != nil {
		// A newer in-memory entry is never deleted, even if another job published it.
		key := openAICodexTicketKey(receipt.accountID, receipt.model)
		if raw, ok := s.openaiCodexTickets.Load(key); ok {
			if ticket, ok := raw.(*openAICodexTicket); ok && receipt.matches(ticket) {
				s.openaiCodexTickets.CompareAndDelete(key, raw)
			}
		}
		s.startCodexAccountTicketJob(ctx, receipt.accountID, false)
	}
	return err == nil
}

const codexTicketWatchdogBufferLimit = 1024 * 1024

// Transparent incremental observer. It never changes response bytes, returns a
// synthetic error, reads ahead, or retries a business request. Oversized/invalid
// frames are ignored rather than interpreted as a routing failure.
type codexTicketWatchdogBody struct {
	io.ReadCloser
	onModelMismatch func(string)
	model           string
	trigger         func(string)
	mode            byte
	buffer          []byte
	data            []byte
	overflow        bool
	skipLine        bool
	eventName       string
	mu              sync.Mutex
	finished        bool
}

func (b *codexTicketWatchdogBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.finished {
		return n, err
	}
	if n > 0 {
		b.observe(p[:n])
	}
	if err == io.EOF {
		b.finishLocked()
	}
	return n, err
}

func (b *codexTicketWatchdogBody) Close() error {
	// Unblock a concurrent underlying Read without holding the parser lock.
	err := b.ReadCloser.Close()
	b.mu.Lock()
	defer b.mu.Unlock()
	b.finishLocked()
	return err
}

// Some streaming consumers stop at the completed data line without reading
// the following blank line or EOF. Finalize only bytes already observed.
func (b *codexTicketWatchdogBody) finishLocked() {
	if b.finished {
		return
	}
	b.finished = true
	if b.mode == 'j' && !b.overflow {
		b.observeJSON(b.buffer)
	} else if b.mode == 's' {
		if len(b.buffer) > 0 && !b.skipLine {
			b.line(b.buffer)
		}
		b.flushEvent()
	}
}

func (b *codexTicketWatchdogBody) observe(p []byte) {
	if b.mode == 0 {
		trimmed := bytes.TrimSpace(p)
		if len(trimmed) == 0 {
			return
		}
		b.mode = 's'
		if trimmed[0] == '{' {
			b.mode = 'j'
		}
	}
	if b.mode == 'j' {
		if !b.overflow && len(b.buffer)+len(p) <= codexTicketWatchdogBufferLimit {
			b.buffer = append(b.buffer, p...)
		} else {
			b.overflow = true
			b.buffer = nil
		}
		return
	}
	for len(p) > 0 {
		end := bytes.IndexByte(p, '\n')
		part := p
		if end >= 0 {
			part = p[:end]
		}
		if !b.skipLine {
			if len(b.buffer)+len(part) > codexTicketWatchdogBufferLimit {
				b.skipLine, b.overflow = true, true
				b.buffer = nil
			} else {
				b.buffer = append(b.buffer, part...)
			}
		}
		if end < 0 {
			return
		}
		if !b.skipLine {
			b.line(b.buffer)
		}
		b.buffer = b.buffer[:0]
		b.skipLine = false
		p = p[end+1:]
	}
}

func (b *codexTicketWatchdogBody) line(line []byte) {
	line = bytes.TrimSuffix(line, []byte{'\r'})
	if len(line) == 0 {
		b.flushEvent()
		return
	}
	if bytes.HasPrefix(line, []byte("event:")) {
		b.eventName = ""
		if strings.TrimSpace(string(line[6:])) == "response.completed" {
			b.eventName = "response.completed"
		}
	}
	if !b.overflow && bytes.HasPrefix(line, []byte("data:")) {
		value := bytes.TrimPrefix(line[5:], []byte{' '})
		if len(b.data)+len(value)+1 > codexTicketWatchdogBufferLimit {
			b.overflow = true
			b.data = nil
		} else {
			b.data = append(b.data, value...)
			b.data = append(b.data, '\n')
		}
	}
}

func (b *codexTicketWatchdogBody) flushEvent() {
	if !b.overflow {
		b.observeJSON(b.data)
	}
	b.data = b.data[:0]
	b.eventName = ""
	b.overflow = false
}

func (b *codexTicketWatchdogBody) observeJSON(raw []byte) {
	if !gjson.ValidBytes(raw) {
		return
	}
	root := gjson.ParseBytes(raw)
	response := root
	eventType := root.Get("type").String()
	completedEvent := eventType == "response.completed" || (eventType == "" && b.eventName == "response.completed")
	if completedEvent {
		response = root.Get("response")
	} else if root.Get("type").Exists() || root.Get("object").String() != "response" {
		return
	}
	status := response.Get("status").String()
	if status != "completed" && !(completedEvent && status == "") {
		return
	}
	actual := response.Get("model")
	if actual.Type != gjson.String {
		return
	}
	model := strings.TrimSpace(actual.String())
	if model != "" && !upstreamModelsMatchForAudit(b.model, model) {
		if b.trigger != nil {
			b.trigger("model_mismatch")
		}
		if b.onModelMismatch != nil {
			b.onModelMismatch(model)
		}
	}
}
