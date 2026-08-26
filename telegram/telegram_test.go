package telegram

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const testToken = "123456:AbCdEf_123"

type recordedRequest struct {
	url         string
	method      string
	contentType string
	body        string
}

type recordingTransport struct {
	mu        sync.Mutex
	requests  []recordedRequest
	responses []http.Response
}

func (transport *recordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	transport.mu.Lock()
	defer transport.mu.Unlock()
	transport.requests = append(transport.requests, recordedRequest{
		url:         request.URL.String(),
		method:      request.Method,
		contentType: request.Header.Get("Content-Type"),
		body:        string(body),
	})
	if len(transport.responses) == 0 {
		return telegramHTTPResponse(http.StatusOK, successResponse(1, "42", "ok")), nil
	}
	response := transport.responses[0]
	transport.responses = transport.responses[1:]
	return &response, nil
}

func telegramHTTPResponse(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

func successResponse(id int64, chatID, text string) string {
	return `{"ok":true,"result":{"message_id":` + strconv.FormatInt(id, 10) + `,"date":1700000000,"chat":{"id":` + chatID + `},"text":` + strconv.Quote(text) + `}}`
}

func resetServices(t *testing.T) {
	t.Helper()
	services.Lock()
	services.byName = make(map[string]*Service)
	services.byTarget = make(map[string]*Service)
	services.byToken = make(map[string]*limiter)
	services.Unlock()
	t.Cleanup(func() {
		services.Lock()
		services.byName = make(map[string]*Service)
		services.byTarget = make(map[string]*Service)
		services.byToken = make(map[string]*limiter)
		services.Unlock()
	})
}

func testOptions(transport *recordingTransport) []Option {
	return []Option{
		WithHTTPClient(&http.Client{Transport: transport}),
		WithBaseURL("https://api.test"),
	}
}

func TestRegisterAndGet(t *testing.T) {
	resetServices(t)
	service, err := Register("alerts", testToken, "-100123", testOptions(&recordingTransport{})...)
	require.NoError(t, err)
	require.Equal(t, "alerts", service.Name())
	require.Equal(t, "-100123", service.ChatID())
	registered, exists := Get("alerts")
	require.True(t, exists)
	require.Same(t, service, registered)

	other, err := Register("reports", testToken, "@reports", testOptions(&recordingTransport{})...)
	require.NoError(t, err)
	require.Same(t, service.botLimiter, other.botLimiter)
	_, err = Register("alerts", "654321:other", "42")
	require.ErrorIs(t, err, ErrRegistered)
	_, err = Register("duplicate", testToken, "-100123")
	require.ErrorIs(t, err, ErrRegistered)
	_, err = Register("bad", "invalid", "42")
	require.ErrorIs(t, err, ErrInvalidToken)
	_, err = Register("bad-chat", testToken, "chat")
	require.ErrorIs(t, err, ErrInvalidChatID)
}

func TestSend(t *testing.T) {
	resetServices(t)
	transport := &recordingTransport{responses: []http.Response{*telegramHTTPResponse(http.StatusOK, successResponse(9, "-100123", "ready"))}}
	service, err := Register("alerts", testToken, "-100123", testOptions(transport)...)
	require.NoError(t, err)

	messages, err := service.Send(context.Background(), "ready")
	require.NoError(t, err)
	require.Equal(t, []Message{{ID: 9, Date: time.Unix(1700000000, 0), ChatID: "-100123", Text: "ready"}}, messages)
	require.Len(t, transport.requests, 1)
	require.Equal(t, "https://api.test/bot"+testToken+"/sendMessage", transport.requests[0].url)
	require.Equal(t, http.MethodPost, transport.requests[0].method)
	require.Equal(t, "application/json", transport.requests[0].contentType)
	require.JSONEq(t, `{"chat_id":"-100123","text":"ready"}`, transport.requests[0].body)

	_, err = service.SendMarkdown(context.Background(), "*ready*")
	require.NoError(t, err)
	require.JSONEq(t, `{"chat_id":"-100123","text":"*ready*","parse_mode":"MarkdownV2"}`, transport.requests[1].body)
	_, err = service.SendHTML(context.Background(), "<b>ready</b>")
	require.NoError(t, err)
	require.JSONEq(t, `{"chat_id":"-100123","text":"<b>ready</b>","parse_mode":"HTML"}`, transport.requests[2].body)
}

func TestSendReturnsPartialResults(t *testing.T) {
	resetServices(t)
	transport := &recordingTransport{responses: []http.Response{
		*telegramHTTPResponse(http.StatusOK, successResponse(1, "42", strings.Repeat("a", TextLimit))),
		*telegramHTTPResponse(http.StatusOK, `{"ok":false,"error_code":400,"description":"Bad Request"}`),
	}}
	service, err := Register("alerts", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)

	messages, err := service.Send(context.Background(), strings.Repeat("a", TextLimit+1))
	require.Len(t, messages, 1)
	var apiError *APIError
	require.ErrorAs(t, err, &apiError)
	require.Equal(t, 400, apiError.Code)
	require.Len(t, transport.requests, 2)
}

func TestSendRetriesFloodControlOnce(t *testing.T) {
	resetServices(t)
	transport := &recordingTransport{responses: []http.Response{
		*telegramHTTPResponse(http.StatusOK, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`),
		*telegramHTTPResponse(http.StatusOK, successResponse(2, "42", "retry")),
	}}
	service, err := Register("alerts", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)

	messages, err := service.Send(context.Background(), "retry")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Len(t, transport.requests, 2)
}

func TestSendDoesNotExposeTokenInErrors(t *testing.T) {
	resetServices(t)
	transport := &recordingTransport{responses: []http.Response{*telegramHTTPResponse(http.StatusBadGateway, "unavailable")}}
	service, err := Register("alerts", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)

	_, err = service.Send(context.Background(), "message")
	var statusError *HTTPStatusError
	require.ErrorAs(t, err, &statusError)
	require.NotContains(t, err.Error(), testToken)
}
