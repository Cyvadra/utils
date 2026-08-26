package telegram

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Cyvadra/utils/internal/testutil"
	"github.com/stretchr/testify/require"
)

const testToken = "123456:AbCdEf_123"

func successResponse(id int64, chatID, text string) string {
	return `{"ok":true,"result":{"message_id":` + strconv.FormatInt(id, 10) + `,"date":1700000000,"chat":{"id":` + chatID + `},"text":` + strconv.Quote(text) + `}}`
}

func newTransport(responses ...*http.Response) *testutil.RecordingTransport {
	return &testutil.RecordingTransport{
		Responses:      responses,
		FallbackStatus: http.StatusOK,
		FallbackBody:   successResponse(1, "42", "ok"),
	}
}

func resetServices(t *testing.T) {
	t.Helper()
	services.byName.Reset()
	services.byTarget.Reset()
	services.byToken.Reset()
	t.Cleanup(func() {
		services.byName.Reset()
		services.byTarget.Reset()
		services.byToken.Reset()
	})
}

func testOptions(transport *testutil.RecordingTransport) []Option {
	return []Option{
		WithHTTPClient(&http.Client{Transport: transport}),
		WithBaseURL("https://api.test"),
	}
}

func TestRegisterAndGet(t *testing.T) {
	resetServices(t)
	service, err := Register("alerts", testToken, "-100123", testOptions(newTransport())...)
	require.NoError(t, err)
	require.Equal(t, "alerts", service.Name())
	require.Equal(t, "-100123", service.ChatID())
	registered, exists := Get("alerts")
	require.True(t, exists)
	require.Same(t, service, registered)
	trimmed, exists := Get("  alerts  ")
	require.True(t, exists)
	require.Same(t, service, trimmed)

	other, err := Register("reports", testToken, "@reports", testOptions(newTransport())...)
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

func TestRegisterRejectsConflictingBotRateLimit(t *testing.T) {
	resetServices(t)
	service, err := Register("alerts", testToken, "-100123", testOptions(newTransport())...)
	require.NoError(t, err)
	_, err = Register("conflict", testToken, "42", WithBotRateLimit(60, time.Second))
	require.ErrorIs(t, err, ErrBotRateLimitConflict)

	matching, err := Register("matching", testToken, "43", WithBotRateLimit(DefaultBotRateLimit, DefaultBotRateInterval))
	require.NoError(t, err)
	require.Same(t, service.botLimiter, matching.botLimiter)
}

func TestSend(t *testing.T) {
	resetServices(t)
	transport := newTransport(testutil.Response(http.StatusOK, successResponse(9, "-100123", "ready")))
	service, err := Register("alerts", testToken, "-100123", testOptions(transport)...)
	require.NoError(t, err)

	messages, err := service.Send(context.Background(), "ready")
	require.NoError(t, err)
	require.Equal(t, []Message{{ID: 9, Date: time.Unix(1700000000, 0), ChatID: "-100123", Text: "ready"}}, messages)
	require.Len(t, transport.Requests, 1)
	require.Equal(t, "https://api.test/bot"+testToken+"/sendMessage", transport.Requests[0].URL)
	require.Equal(t, http.MethodPost, transport.Requests[0].Method)
	require.Equal(t, "application/json", transport.Requests[0].ContentType)
	require.JSONEq(t, `{"chat_id":"-100123","text":"ready"}`, transport.Requests[0].Body)

	_, err = service.SendMarkdown(context.Background(), "*ready*")
	require.NoError(t, err)
	require.JSONEq(t, `{"chat_id":"-100123","text":"*ready*","parse_mode":"MarkdownV2"}`, transport.Requests[1].Body)
	_, err = service.SendHTML(context.Background(), "<b>ready</b>")
	require.NoError(t, err)
	require.JSONEq(t, `{"chat_id":"-100123","text":"<b>ready</b>","parse_mode":"HTML"}`, transport.Requests[2].Body)
}

func TestSendReturnsPartialResults(t *testing.T) {
	resetServices(t)
	transport := newTransport(
		testutil.Response(http.StatusOK, successResponse(1, "42", strings.Repeat("a", TextLimit))),
		testutil.Response(http.StatusOK, `{"ok":false,"error_code":400,"description":"Bad Request"}`),
	)
	service, err := Register("alerts", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)

	messages, err := service.Send(context.Background(), strings.Repeat("a", TextLimit+1))
	require.Len(t, messages, 1)
	var apiError *APIError
	require.ErrorAs(t, err, &apiError)
	require.Equal(t, 400, apiError.Code)
	require.Len(t, transport.Requests, 2)
}

func TestSendRetriesFloodControlOnce(t *testing.T) {
	resetServices(t)
	transport := newTransport(
		testutil.Response(http.StatusOK, `{"ok":false,"error_code":429,"description":"Too Many Requests","parameters":{"retry_after":1}}`),
		testutil.Response(http.StatusOK, successResponse(2, "42", "retry")),
	)
	service, err := Register("alerts", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)

	messages, err := service.Send(context.Background(), "retry")
	require.NoError(t, err)
	require.Len(t, messages, 1)
	require.Len(t, transport.Requests, 2)
}

func TestSendDoesNotExposeTokenInErrors(t *testing.T) {
	resetServices(t)
	transport := newTransport(testutil.Response(http.StatusBadGateway, "unavailable"))
	service, err := Register("alerts", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)

	_, err = service.Send(context.Background(), "message")
	var statusError *HTTPStatusError
	require.ErrorAs(t, err, &statusError)
	require.NotContains(t, err.Error(), testToken)
}

func TestSendWaitsForRateLimit(t *testing.T) {
	resetServices(t)
	transport := newTransport()
	service, err := Register("limited", testToken, "42",
		WithHTTPClient(&http.Client{Transport: transport}),
		WithBaseURL("https://api.test"),
		WithChatRateLimit(1, time.Hour),
	)
	require.NoError(t, err)
	_, err = service.Send(context.Background(), "first")
	require.NoError(t, err)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = service.Send(ctx, "second")
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, transport.Requests, 1)
}
