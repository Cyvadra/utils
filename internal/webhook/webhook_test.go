package webhook

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSendJSONSuccess(t *testing.T) {
	var method, contentType string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		method = r.Method
		contentType = r.Header.Get("Content-Type")
		w.Write([]byte(`{"ok":true}`))
	}))
	defer server.Close()

	var result struct {
		OK bool `json:"ok"`
	}
	require.NoError(t, SendJSON(context.Background(), server.Client(), server.URL, map[string]any{"a": 1}, &result))
	require.True(t, result.OK)
	require.Equal(t, http.MethodPost, method)
	require.Equal(t, "application/json", contentType)
}

func TestSendJSONNon2xx(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer server.Close()

	err := SendJSON(context.Background(), server.Client(), server.URL, map[string]any{}, &struct{}{})
	var statusErr *HTTPStatusError
	require.ErrorAs(t, err, &statusErr)
	require.Equal(t, http.StatusBadGateway, statusErr.StatusCode)
}

func TestSendJSONDecodeError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("not json"))
	}))
	defer server.Close()

	err := SendJSON(context.Background(), server.Client(), server.URL, map[string]any{}, &struct{}{})
	var decodeErr *DecodeError
	require.ErrorAs(t, err, &decodeErr)
}

func TestSendJSONRequestError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := server.URL
	server.Close()

	err := SendJSON(context.Background(), server.Client(), url, map[string]any{}, &struct{}{})
	require.Error(t, err)
}
