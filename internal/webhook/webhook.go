// Package webhook provides the shared HTTP plumbing for webhook-based
// notification integrations such as Telegram and WeCom.
package webhook

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// DefaultClient is the default HTTP client used for webhook requests. It applies
// a timeout so a hung connection cannot block a send indefinitely.
var DefaultClient = &http.Client{Timeout: 10 * time.Second}

// HTTPStatusError is returned when a webhook responds with a non-success status.
type HTTPStatusError struct {
	StatusCode int
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("unexpected HTTP status %d", e.StatusCode)
}

// DecodeError is returned when a successful response body cannot be decoded.
type DecodeError struct {
	Err error
}

func (e *DecodeError) Error() string {
	return fmt.Sprintf("decode response: %v", e.Err)
}

func (e *DecodeError) Unwrap() error {
	return e.Err
}

// SendJSON marshals payload and POSTs it to url as JSON, then decodes the
// response body into dst. Errors are as for Send, plus an error wrapping any
// encode failure.
func SendJSON(ctx context.Context, client *http.Client, url string, payload, dst any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode message: %w", err)
	}
	return Send(ctx, client, url, "application/json", body, dst)
}

// Send POSTs body to url with the given content type, then decodes the
// response body into dst. It returns:
//   - *HTTPStatusError for a non-2xx status (the body is drained first)
//   - *DecodeError when the body cannot be decoded into dst
//   - an error wrapping any request or transport failure
func Send(ctx context.Context, client *http.Client, url, contentType string, body []byte, dst any) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	request.Header.Set("Content-Type", contentType)

	response, err := client.Do(request)
	if err != nil {
		return fmt.Errorf("send request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		io.Copy(io.Discard, response.Body)
		return &HTTPStatusError{StatusCode: response.StatusCode}
	}
	if err := json.NewDecoder(response.Body).Decode(dst); err != nil {
		return &DecodeError{Err: err}
	}
	return nil
}
