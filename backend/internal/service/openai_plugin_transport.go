package service

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/Wei-Shaw/sub2api/internal/pkg/tlsfingerprint"
)

func requestModelFromBody(req *http.Request) string {
	if req == nil || req.GetBody == nil {
		return ""
	}
	body, err := req.GetBody()
	if err != nil {
		return ""
	}
	defer func() { _ = body.Close() }()
	raw, err := io.ReadAll(io.LimitReader(body, 1<<20))
	if err != nil {
		return ""
	}
	var payload struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(raw, &payload) != nil {
		return ""
	}
	return payload.Model
}

func (s *OpenAIGatewayService) SetPluginManager(manager *PluginManager) {
	s.pluginManager = manager
}

// doOpenAIUpstream 只在 OpenAI OAuth 能力绑定已启用时把真实请求交给插件。
// 插件返回标准 http.Response，响应解析、错误映射、SSE 和计费仍由现有核心链处理。
func (s *OpenAIGatewayService) doOpenAIUpstream(request *http.Request, proxyURL string, account *Account) (*http.Response, error) {
	request = WithAccountTrafficRequest(request, account)
	response, err := accountTrafficController(s.httpUpstream).DoHTTP(request, func(controlled *http.Request) (*http.Response, error) {
		return s.doOpenAIUpstreamWithoutTraffic(controlled, proxyURL, account)
	})
	if err == nil {
		s.observeCodexTicketResponse(request, response)
	}
	return response, err
}

func (s *OpenAIGatewayService) doOpenAIUpstreamWithoutTraffic(request *http.Request, proxyURL string, account *Account) (*http.Response, error) {
	if err := applyCodexRequestEndpoint(request, s.accountRepo, s.cfg, account); err != nil {
		return nil, err
	}
	if s.pluginManager != nil {
		response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(request.Context(), request, proxyURL, account)
		if handled {
			return response, err
		}
	}
	if s.cfg == nil || s.cfg.Gateway.TLSFingerprint.Enabled {
		if profile, profileErr := resolveMode1TLSProfile(account); profileErr != nil {
			return nil, profileErr
		} else if profile != nil {
			return s.httpUpstream.DoWithTLS(request, proxyURL, account.ID, account.Concurrency, profile)
		}
	}
	return s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
}

// doOpenAIAccountTestUpstream 让 OpenAI OAuth 账号测试与真实转发使用同一插件路径。
// API Key 和未命中插件的账号保持各自原有的 HTTPUpstream 行为。
func (s *AccountTestService) doOpenAIAccountTestUpstream(
	request *http.Request,
	proxyURL string,
	account *Account,
	useTLSFallback bool,
) (response *http.Response, err error) {
	request = WithAccountTrafficRequest(request, account)
	return accountTrafficController(s.httpUpstream).DoHTTP(request, func(controlled *http.Request) (*http.Response, error) {
		return s.doOpenAIAccountTestUpstreamWithoutTraffic(controlled, proxyURL, account, useTLSFallback)
	})
}

func (s *AccountTestService) doOpenAIAccountTestUpstreamWithoutTraffic(
	request *http.Request,
	proxyURL string,
	account *Account,
	useTLSFallback bool,
) (response *http.Response, err error) {
	if err := applyCodexRequestEndpoint(request, s.accountRepo, s.cfg, account); err != nil {
		return nil, err
	}
	// 账号测试必须与真实 Codex HTTP/Responses 调度共享同一 STATE 门禁。
	// 未配置 gateway 时保留官方账号测试路径；配置后由 gateway 读取实时账号、
	// 校验固定代理/凭据指纹并注入已验证票据，失败时 fail-closed 且不拨号。
	if s.openaiGatewayService != nil {
		model := requestModelFromBody(request)
		stateRequired := isOpenAICodexTicketAccount(account) && codexAccountTicketConfigOf(account).Enabled
		if model == "" && stateRequired {
			return nil, errors.Join(ErrOpenAICodexTicketUnavailable, errors.New("STATE request model/body is unavailable"))
		}
		if stateRequired && proxyURL != "" && account != nil && account.Proxy != nil && proxyURL != account.Proxy.URL() {
			return nil, errors.Join(ErrOpenAICodexTicketUnavailable, errors.New("STATE fixed business route mismatch"))
		}
		if err := s.openaiGatewayService.applyOpenAICodexTicketToRequest(request.Context(), account, model, request); err != nil {
			return nil, err
		}
	}
	if observation := codexObservation(request.Context()); observation != nil {
		observation.result.TargetURL = safeCodexDiagnosticURL(request.URL.String())
		observation.emit()
		defer func() {
			if response != nil {
				observation.result.HTTPStatus = response.StatusCode
				if response.Request != nil && response.Request.URL != nil {
					observation.result.TargetURL = safeCodexDiagnosticURL(response.Request.URL.String())
				}
			}
			observation.emit()
		}()
	}
	if s.pluginManager != nil {
		response, handled, err := s.pluginManager.RoundTripOpenAIOAuth(request.Context(), request, proxyURL, account)
		if handled {
			if s.openaiGatewayService != nil {
				s.openaiGatewayService.observeCodexTicketResponse(request, response)
			}
			s.observeAccountTestModelMismatch(request, response, account)
			return response, err
		}
	}
	if isOpenAICodexTicketAccount(account) && codexAccountTicketConfigOf(account).Enabled && s.openaiGatewayService == nil {
		return nil, ErrOpenAICodexTicketUnavailable
	}
	if useTLSFallback {
		response, err = s.httpUpstream.DoWithTLS(
			request,
			proxyURL,
			account.ID,
			account.Concurrency,
			resolveAccountTLSProfile(s.tlsFPProfileService, account),
		)
	} else if profile, profileErr := resolveMode1TLSProfile(account); profileErr != nil {
		return nil, profileErr
	} else if profile != nil {
		response, err = s.httpUpstream.DoWithTLS(request, proxyURL, account.ID, account.Concurrency, profile)
	} else {
		response, err = s.httpUpstream.Do(request, proxyURL, account.ID, account.Concurrency)
	}
	if s.openaiGatewayService != nil {
		s.openaiGatewayService.observeCodexTicketResponse(request, response)
	}
	s.observeAccountTestModelMismatch(request, response, account)
	return response, err
}

func resolveAccountTLSProfile(service *TLSFingerprintProfileService, account *Account) *tlsfingerprint.Profile {
	if service != nil {
		return service.ResolveTLSProfile(account)
	}
	profile, _ := resolveMode1TLSProfile(account)
	return profile
}
