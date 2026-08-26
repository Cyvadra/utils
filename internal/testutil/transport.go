// Package testutil provides shared helpers for testing webhook integrations.
package testutil

import (
	"io"
	"net/http"
	"strings"
	"sync"
)

// RecordedRequest captures a single request made through a RecordingTransport.
type RecordedRequest struct {
	URL         string
	Method      string
	ContentType string
	Body        string
}

// RecordingTransport records every request it handles and replays queued
// responses in order, falling back to a configurable default response.
type RecordingTransport struct {
	mu             sync.Mutex
	Requests       []RecordedRequest
	Responses      []*http.Response
	FallbackStatus int
	FallbackBody   string
}

// RoundTrip records the request and returns the next queued response, or the
// configured fallback when the queue is empty.
func (t *RecordingTransport) RoundTrip(request *http.Request) (*http.Response, error) {
	body, err := io.ReadAll(request.Body)
	if err != nil {
		return nil, err
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.Requests = append(t.Requests, RecordedRequest{
		URL:         request.URL.String(),
		Method:      request.Method,
		ContentType: request.Header.Get("Content-Type"),
		Body:        string(body),
	})
	if len(t.Responses) == 0 {
		return Response(t.FallbackStatus, t.FallbackBody), nil
	}
	response := t.Responses[0]
	t.Responses = t.Responses[1:]
	return response, nil
}

// Response builds an *http.Response with the given status and body.
func Response(statusCode int, body string) *http.Response {
	return &http.Response{
		StatusCode: statusCode,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}
