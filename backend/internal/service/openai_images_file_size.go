package service

import (
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/tidwall/gjson"
)

const OpenAIImagesFileTooLargeCode = "image_file_too_large"

// Match the provider's file-byte error, never generic capacity/token limits.
var openAIImagesFileByteLimit = regexp.MustCompile(`(?i)file\s+"(?:request_file:)?(image(?:\[[0-9]*\])?|mask)"\s+exceeds the\s+([0-9]+)\s+byte limit`)
var openAIImagesFileParam = regexp.MustCompile(`^(?:request_file:)?(image(?:\[[0-9]*\])?|mask)$`)

func openAIImagesFileSizeError(body []byte) *OpenAIImagesUpstreamError {
	message := extractUpstreamErrorMessage(body)
	code := strings.ToLower(strings.TrimSpace(extractUpstreamErrorCode(body)))
	param := strings.TrimSpace(gjson.GetBytes(body, "error.param").String())
	field := openAIImagesFileParam.FindStringSubmatch(param)
	match := openAIImagesFileByteLimit.FindStringSubmatch(message)
	explicitCode := code == OpenAIImagesFileTooLargeCode || code == "image_too_large" ||
		(code == "file_too_large" && len(field) > 0)
	if len(match) == 0 && !explicitCode {
		return nil
	}
	param = ""
	if len(field) > 0 {
		param = field[1]
	}
	clientMessage := "参考图片超过当前上游的文件大小限制，请压缩或替换图片后重试。"
	if len(match) > 0 {
		param = match[1]
		if limit, err := strconv.ParseUint(match[2], 10, 64); err == nil && limit > 0 {
			clientMessage = fmt.Sprintf("参考图片超过当前上游的文件大小限制（%d 字节，%.2f MiB），请压缩或替换图片后重试。", limit, float64(limit)/(1<<20))
		}
	}
	return &OpenAIImagesUpstreamError{
		StatusCode: http.StatusRequestEntityTooLarge, ErrorType: "invalid_request_error",
		Code: OpenAIImagesFileTooLargeCode, Message: clientMessage, Param: param,
	}
}

// Classify before retry/health side effects, retaining the provider's original
// status and sanitized explanation only in ops diagnostics.
func (s *OpenAIGatewayService) handleOpenAIImagesFileSizeError(c *gin.Context, account *Account, resp *http.Response, body []byte) *OpenAIImagesUpstreamError {
	inputErr := openAIImagesFileSizeError(body)
	if inputErr == nil {
		return nil
	}
	inputErr.UpstreamRequestID = strings.TrimSpace(resp.Header.Get("x-request-id"))
	message := sanitizeUpstreamErrorMessage(extractUpstreamErrorMessage(body))
	setOpsUpstreamError(c, resp.StatusCode, message, "")
	appendOpsUpstreamError(c, OpsUpstreamErrorEvent{
		ProxyID: opsUpstreamProxyID(account), ProxyName: opsUpstreamProxyName(account),
		Platform: account.Platform, AccountID: account.ID, AccountName: account.Name,
		UpstreamStatusCode: resp.StatusCode, UpstreamRequestID: inputErr.UpstreamRequestID,
		Kind: "http_error", Message: message,
	})
	if !writeOpenAIImagesUpstreamErrorResponse(c, inputErr) && strings.HasPrefix(c.Writer.Header().Get("Content-Type"), "text/event-stream") {
		_ = s.writeOpenAIImagesStreamEvent(c, c.Writer, "error", buildOpenAIImagesStreamErrorBodyFromUpstream(inputErr))
	}
	if c.Writer.Status() < 400 {
		MarkOpsStreamErrorValue(c, OpsStreamError{
			ErrType: inputErr.ErrorType, Code: inputErr.Code, Message: inputErr.Message,
			IntendedStatus: inputErr.StatusCode, CountTowardsSLA: true,
			NonStream: !strings.HasPrefix(c.Writer.Header().Get("Content-Type"), "text/event-stream"),
		})
	}
	return inputErr
}
