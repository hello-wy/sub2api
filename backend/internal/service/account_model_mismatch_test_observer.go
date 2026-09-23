package service

import (
	"context"
	"io"
	"net/http"
	"sync"
)

// Observe only a completed Responses declaration. This reader never consumes
// ahead, changes result bytes/status, or derives models from generated text.
func (s *AccountTestService) observeAccountTestModelMismatch(req *http.Request, resp *http.Response, account *Account) {
	if s == nil || req == nil || req.GetBody == nil || account == nil || account.ID <= 0 || resp == nil || resp.Body == nil || resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return
	}
	repo := s.accountRepo
	if repo == nil && s.openaiGatewayService != nil {
		repo = s.openaiGatewayService.accountRepo
	}
	if _, supported := repo.(AccountModelMismatchMarker); !supported {
		return
	}
	body, err := req.GetBody()
	if err != nil || body == nil {
		return
	}
	const maxBody = 16 << 20
	payload, err := io.ReadAll(io.LimitReader(body, maxBody+1))
	_ = body.Close()
	if err != nil || len(payload) > maxBody {
		return
	}
	model := extractOpenAICodexTicketModel(payload)
	if model == "" {
		return
	}
	accountID, requestID := account.ID, resp.Header.Get("x-request-id")
	observationCtx := WithModelMismatchCredentialExpectation(openAIAccountTestAuthContext(req), account)
	var once sync.Once
	callback := func(actual string) {
		once.Do(func() {
			// Persistence has its own bounded deadline; never block the test's
			// streaming reader while the database or scheduler cache is slow.
			go quarantineAccountModelMismatch(context.WithoutCancel(observationCtx), repo, &Account{ID: accountID}, model, actual, requestID)
		})
	}
	if observer, ok := resp.Body.(*codexTicketWatchdogBody); ok {
		observer.onModelMismatch = callback
	} else {
		resp.Body = &codexTicketWatchdogBody{ReadCloser: resp.Body, model: model, onModelMismatch: callback}
	}
}
