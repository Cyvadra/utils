package wecom

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type recordedRequest struct {
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
		method:      request.Method,
		contentType: request.Header.Get("Content-Type"),
		body:        string(body),
	})
	if len(transport.responses) == 0 {
		return response(http.StatusOK, `{"errcode":0,"errmsg":"ok"}`), nil
	}
	result := transport.responses[0]
	transport.responses = transport.responses[1:]
	return &result, nil
}

func response(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Body:       io.NopCloser(strings.NewReader(body)),
		Header:     make(http.Header),
	}
}

func resetServices(t *testing.T) {
	t.Helper()
	services.Lock()
	services.byName = make(map[string]*Service)
	services.byWebhook = make(map[string]*Service)
	services.Unlock()
	t.Cleanup(func() {
		services.Lock()
		services.byName = make(map[string]*Service)
		services.byWebhook = make(map[string]*Service)
		services.Unlock()
	})
}

func testWebhook(key string) string {
	return "https://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=" + key
}

func TestRegisterAndGet(t *testing.T) {
	resetServices(t)
	client := &http.Client{Transport: &recordingTransport{}}

	service, err := Register("alerts", testWebhook("first"), WithHTTPClient(client))
	require.NoError(t, err)
	require.Equal(t, "alerts", service.Name())
	registered, exists := Get("alerts")
	require.True(t, exists)
	require.Same(t, service, registered)

	_, err = Register("alerts", testWebhook("second"))
	require.ErrorIs(t, err, ErrBotRegistered)
	_, err = Register("other", testWebhook("first"))
	require.ErrorIs(t, err, ErrBotRegistered)
	_, err = Register("invalid", "http://qyapi.weixin.qq.com/cgi-bin/webhook/send?key=key")
	require.ErrorIs(t, err, ErrInvalidWebhookURL)
}

func TestSendTextWithMentions(t *testing.T) {
	resetServices(t)
	transport := &recordingTransport{}
	service, err := Register("alerts", testWebhook("text"), WithHTTPClient(&http.Client{Transport: transport}))
	require.NoError(t, err)

	err = service.Send(context.Background(), "deploy complete", Mention{
		UserIDs:       []string{"alice"},
		MobileNumbers: []string{"13800001111"},
		All:           true,
	})
	require.NoError(t, err)
	require.Len(t, transport.requests, 1)
	require.Equal(t, http.MethodPost, transport.requests[0].method)
	require.Equal(t, "application/json", transport.requests[0].contentType)
	require.JSONEq(t, `{
		"msgtype":"text",
		"text":{
			"content":"deploy complete",
			"mentioned_list":["alice","@all"],
			"mentioned_mobile_list":["13800001111"]
		}
	}`, transport.requests[0].body)
}

func TestSendMarkdownSplitsLongContent(t *testing.T) {
	resetServices(t)
	transport := &recordingTransport{}
	service, err := Register("reports", testWebhook("markdown"), WithHTTPClient(&http.Client{Transport: transport}))
	require.NoError(t, err)
	content := strings.Repeat("a", MarkdownLimit-2) + "\n" + strings.Repeat("b", 10)

	require.NoError(t, service.SendMarkdown(context.Background(), content))
	require.Len(t, transport.requests, 2)
	for _, request := range transport.requests {
		require.LessOrEqual(t, len(extractContent(t, request.body)), MarkdownLimit)
	}
	require.Equal(t, content, extractContent(t, transport.requests[0].body)+extractContent(t, transport.requests[1].body))
}

func TestSendStopsOnWebhookFailure(t *testing.T) {
	resetServices(t)
	transport := &recordingTransport{responses: []http.Response{
		*response(http.StatusOK, `{"errcode":0,"errmsg":"ok"}`),
		*response(http.StatusOK, `{"errcode":93000,"errmsg":"rate limited"}`),
	}}
	service, err := Register("failing", testWebhook("failure"), WithHTTPClient(&http.Client{Transport: transport}))
	require.NoError(t, err)

	err = service.Send(context.Background(), strings.Repeat("a", TextLimit*2+1))
	var apiError *APIError
	require.ErrorAs(t, err, &apiError)
	require.Equal(t, 93000, apiError.Code)
	require.Len(t, transport.requests, 2)
}

func TestSendReturnsContextErrorWhenRateLimited(t *testing.T) {
	resetServices(t)
	transport := &recordingTransport{}
	service, err := Register(
		"limited",
		testWebhook("limited"),
		WithHTTPClient(&http.Client{Transport: transport}),
		WithRateLimit(1, time.Hour),
	)
	require.NoError(t, err)
	require.NoError(t, service.Send(context.Background(), "first"))

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = service.Send(ctx, "second")
	require.ErrorIs(t, err, context.Canceled)
	require.Len(t, transport.requests, 1)
}

func TestSendReturnsHTTPStatusError(t *testing.T) {
	resetServices(t)
	transport := &recordingTransport{responses: []http.Response{*response(http.StatusBadGateway, "unavailable")}}
	service, err := Register("unavailable", testWebhook("unavailable"), WithHTTPClient(&http.Client{Transport: transport}))
	require.NoError(t, err)

	err = service.Send(context.Background(), "message")
	var statusError *HTTPStatusError
	require.ErrorAs(t, err, &statusError)
	require.Equal(t, http.StatusBadGateway, statusError.StatusCode)
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
