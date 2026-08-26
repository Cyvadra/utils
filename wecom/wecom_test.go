package wecom

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Cyvadra/utils/internal/testutil"
	"github.com/stretchr/testify/require"
)

func newTransport(responses ...*http.Response) *testutil.RecordingTransport {
	return &testutil.RecordingTransport{
		Responses:      responses,
		FallbackStatus: http.StatusOK,
		FallbackBody:   `{"errcode":0,"errmsg":"ok"}`,
	}
}

func resetServices(t *testing.T) {
	t.Helper()
	services.byName.Reset()
	services.byWebhook.Reset()
	t.Cleanup(func() {
		services.byName.Reset()
		services.byWebhook.Reset()
	})
}

func testWebhook(key string) string {
	return "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + key
}

func TestRegisterAndGet(t *testing.T) {
	resetServices(t)
	client := &http.Client{Transport: newTransport()}

	service, err := Register("alerts", testWebhook("first"), WithHTTPClient(client))
	require.NoError(t, err)
	require.Equal(t, "alerts", service.Name())
	registered, exists := Get("alerts")
	require.True(t, exists)
	require.Same(t, service, registered)
	trimmed, exists := Get("  alerts  ")
	require.True(t, exists)
	require.Same(t, service, trimmed)

	_, err = Register("alerts", testWebhook("second"))
	require.ErrorIs(t, err, ErrBotRegistered)
	_, err = Register("other", testWebhook("first"))
	require.ErrorIs(t, err, ErrBotRegistered)
	_, err = Register("invalid", "http://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=key")
	require.ErrorIs(t, err, ErrInvalidWebhookURL)
}

func TestSendTextWithMentions(t *testing.T) {
	resetServices(t)
	transport := newTransport()
	service, err := Register("alerts", testWebhook("text"), WithHTTPClient(&http.Client{Transport: transport}))
	require.NoError(t, err)

	delivered, err := service.Send(context.Background(), "deploy complete", Mention{
		UserIDs:       []string{"alice"},
		MobileNumbers: []string{"13800001111"},
		All:           true,
	})
	require.NoError(t, err)
	require.Equal(t, 1, delivered)
	require.Len(t, transport.Requests, 1)
	require.Equal(t, http.MethodPost, transport.Requests[0].Method)
	require.Equal(t, "application/json", transport.Requests[0].ContentType)
	require.JSONEq(t, `{
		"msgtype":"text",
		"text":{
			"content":"deploy complete",
			"mentioned_list":["alice","@all"],
			"mentioned_mobile_list":["13800001111"]
		}
	}`, transport.Requests[0].Body)
}

func TestSendMarkdownSplitsLongContent(t *testing.T) {
	resetServices(t)
	transport := newTransport()
	service, err := Register("reports", testWebhook("markdown"), WithHTTPClient(&http.Client{Transport: transport}))
	require.NoError(t, err)
	content := strings.Repeat("a", MarkdownLimit-2) + "\n" + strings.Repeat("b", 10)

	delivered, err := service.SendMarkdown(context.Background(), content)
	require.NoError(t, err)
	require.Equal(t, 2, delivered)
	require.Len(t, transport.Requests, 2)
	for _, request := range transport.Requests {
		require.LessOrEqual(t, len(extractContent(t, request.Body)), MarkdownLimit)
	}
	require.Equal(t, content, extractContent(t, transport.Requests[0].Body)+extractContent(t, transport.Requests[1].Body))
}

func TestSendStopsOnWebhookFailure(t *testing.T) {
	resetServices(t)
	transport := newTransport(
		testutil.Response(http.StatusOK, `{"errcode":0,"errmsg":"ok"}`),
		testutil.Response(http.StatusOK, `{"errcode":93000,"errmsg":"rate limited"}`),
	)
	service, err := Register("failing", testWebhook("failure"), WithHTTPClient(&http.Client{Transport: transport}))
	require.NoError(t, err)

	delivered, err := service.Send(context.Background(), strings.Repeat("a", TextLimit*2+1))
	require.Equal(t, 1, delivered)
	var apiError *APIError
	require.ErrorAs(t, err, &apiError)
	require.Equal(t, 93000, apiError.Code)
	require.Len(t, transport.Requests, 2)
}

func TestSendReturnsContextErrorWhenRateLimited(t *testing.T) {
	resetServices(t)
	transport := newTransport()
	service, err := Register(
		"limited",
		testWebhook("limited"),
		WithHTTPClient(&http.Client{Transport: transport}),
		WithRateLimit(1, time.Hour),
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

func TestSendReturnsHTTPStatusError(t *testing.T) {
	resetServices(t)
	transport := newTransport(testutil.Response(http.StatusBadGateway, "unavailable"))
	service, err := Register("unavailable", testWebhook("unavailable"), WithHTTPClient(&http.Client{Transport: transport}))
	require.NoError(t, err)

	_, err = service.Send(context.Background(), "message")
	var statusError *HTTPStatusError
	require.ErrorAs(t, err, &statusError)
	require.Equal(t, http.StatusBadGateway, statusError.StatusCode)
}

func TestSendRejectsEmptyContent(t *testing.T) {
	resetServices(t)
	service, err := Register("empty", testWebhook("empty"), WithHTTPClient(&http.Client{Transport: newTransport()}))
	require.NoError(t, err)

	_, err = service.Send(context.Background(), "")
	require.ErrorIs(t, err, ErrEmptyContent)
	_, err = service.Send(context.Background(), "   ")
	require.ErrorIs(t, err, ErrEmptyContent)
	_, err = service.SendMarkdown(context.Background(), "")
	require.ErrorIs(t, err, ErrEmptyContent)
}

func TestSendMentionsFirstChunkOnly(t *testing.T) {
	resetServices(t)
	transport := newTransport()
	service, err := Register("mentions", testWebhook("mentions"), WithHTTPClient(&http.Client{Transport: transport}))
	require.NoError(t, err)
	content := strings.Repeat("a", TextLimit*2+1)

	delivered, err := service.Send(context.Background(), content, Mention{UserIDs: []string{"alice"}, All: true})
	require.NoError(t, err)
	require.Equal(t, 3, delivered)
	require.Len(t, transport.Requests, 3)
	require.JSONEq(t, `{"msgtype":"text","text":{"content":`+strconv.Quote(strings.Repeat("a", TextLimit))+`,"mentioned_list":["alice","@all"]}}`, transport.Requests[0].Body)
	require.JSONEq(t, `{"msgtype":"text","text":{"content":`+strconv.Quote(strings.Repeat("a", TextLimit))+`}}`, transport.Requests[1].Body)
	require.JSONEq(t, `{"msgtype":"text","text":{"content":"a"}}`, transport.Requests[2].Body)
}

func TestMergeMentionsDeduplicates(t *testing.T) {
	merged := mergeMentions([]Mention{
		{UserIDs: []string{"alice", "bob"}, MobileNumbers: []string{"13800001111"}},
		{UserIDs: []string{"bob", "@all"}, All: true},
	})
	require.Equal(t, []string{"alice", "bob", "@all"}, merged.UserIDs)
	require.Equal(t, []string{"13800001111"}, merged.MobileNumbers)
}

func extractContent(t *testing.T, body string) string {
	t.Helper()
	var payload struct {
		Text struct {
			Content string `json:"content"`
		} `json:"text"`
		Markdown struct {
			Content string `json:"content"`
		} `json:"markdown"`
	}
	require.NoError(t, json.Unmarshal([]byte(body), &payload))
	if payload.Text.Content != "" {
		return payload.Text.Content
	}
	return payload.Markdown.Content
}
