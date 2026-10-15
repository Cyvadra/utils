package telegram

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Cyvadra/utils/internal/webhook"
)

type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type telegramResponse struct {
	OK          bool            `json:"ok"`
	Result      json.RawMessage `json:"result"`
	ErrorCode   int             `json:"error_code"`
	Description string          `json:"description"`
	Parameters  struct {
		RetryAfter      int64           `json:"retry_after"`
		MigrateToChatID json.RawMessage `json:"migrate_to_chat_id"`
	} `json:"parameters"`
}

type messageResult struct {
	ID   int64 `json:"message_id"`
	Date int64 `json:"date"`
	Chat struct {
		ID json.RawMessage `json:"id"`
	} `json:"chat"`
	Text    string `json:"text"`
	Caption string `json:"caption"`
}

func (result messageResult) message() Message {
	text := result.Text
	if text == "" {
		text = result.Caption
	}
	return Message{
		ID:     result.ID,
		Date:   time.Unix(result.Date, 0),
		ChatID: chatIDFromJSON(result.Chat.ID),
		Text:   text,
	}
}

// request is one Bot API call, kept as encoded bytes so it can be retried.
type request struct {
	method      string
	contentType string
	body        []byte
}

// Send sends plain text to the service's default chat.
func (service *Service) Send(ctx context.Context, text string) ([]Message, error) {
	chunks, err := splitText(text)
	if err != nil {
		return nil, err
	}
	return service.sendChunks(ctx, chunks, "")
}

// SendMarkdown sends MarkdownV2 text to the service's default chat.
func (service *Service) SendMarkdown(ctx context.Context, markdown string) ([]Message, error) {
	chunks, err := splitMarkdown(markdown)
	if err != nil {
		return nil, err
	}
	return service.sendChunks(ctx, chunks, "MarkdownV2")
}

// SendHTML sends Telegram-supported HTML to the service's default chat.
func (service *Service) SendHTML(ctx context.Context, html string) ([]Message, error) {
	chunks, err := splitHTML(html)
	if err != nil {
		return nil, err
	}
	return service.sendChunks(ctx, chunks, "HTML")
}

func (service *Service) sendChunks(ctx context.Context, chunks []string, parseMode string) ([]Message, error) {
	messages := make([]Message, 0, len(chunks))
	for index, chunk := range chunks {
		message, err := service.sendChunk(ctx, sendMessageRequest{
			ChatID:    service.chatID,
			Text:      chunk,
			ParseMode: parseMode,
		})
		if err != nil {
			return messages, fmt.Errorf("telegram: send chunk %d of %d: %w", index+1, len(chunks), err)
		}
		messages = append(messages, message)
	}
	return messages, nil
}

func (service *Service) sendChunk(ctx context.Context, payload sendMessageRequest) (Message, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("telegram: encode message: %w", err)
	}
	var result messageResult
	if err := service.call(ctx, request{method: "sendMessage", contentType: "application/json", body: body}, &result); err != nil {
		return Message{}, err
	}
	return result.message(), nil
}

// call sends req, retrying once when Telegram asks to back off, and decodes
// the response's result into dst.
func (service *Service) call(ctx context.Context, req request, dst any) error {
	for attempt := 0; attempt < 2; attempt++ {
		if err := service.wait(ctx); err != nil {
			return err
		}
		err := service.dispatch(ctx, req, dst)
		if err == nil {
			return nil
		}
		var apiError *APIError
		if attempt == 0 && errors.As(err, &apiError) && apiError.Code == http.StatusTooManyRequests && apiError.RetryAfter > 0 {
			if err := waitDuration(ctx, apiError.RetryAfter); err != nil {
				return err
			}
			continue
		}
		return err
	}
	return errors.New("telegram: retry attempts exhausted")
}

func (service *Service) wait(ctx context.Context) error {
	if err := service.botLimiter.Wait(ctx); err != nil {
		return err
	}
	return service.chatLimiter.Wait(ctx)
}

func (service *Service) dispatch(ctx context.Context, req request, dst any) error {
	url := service.baseURL + "/bot" + service.token + "/" + req.method
	var result telegramResponse
	if err := webhook.Send(ctx, service.client, url, req.contentType, req.body, &result); err != nil {
		var statusErr *webhook.HTTPStatusError
		if errors.As(err, &statusErr) {
			return &HTTPStatusError{StatusCode: statusErr.StatusCode}
		}
		var decodeErr *webhook.DecodeError
		if errors.As(err, &decodeErr) {
			return fmt.Errorf("%w: %v", ErrInvalidResponse, decodeErr.Err)
		}
		return fmt.Errorf("telegram: %w", err)
	}
	if !result.OK {
		return &APIError{
			Code:            result.ErrorCode,
			Description:     result.Description,
			RetryAfter:      time.Duration(result.Parameters.RetryAfter) * time.Second,
			MigrateToChatID: chatIDFromJSON(result.Parameters.MigrateToChatID),
		}
	}
	if err := json.Unmarshal(result.Result, dst); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	return nil
}

func chatIDFromJSON(value json.RawMessage) string {
	if len(value) == 0 || string(value) == "null" {
		return ""
	}
	var stringValue string
	if err := json.Unmarshal(value, &stringValue); err == nil {
		return stringValue
	}
	return strings.TrimSpace(string(value))
}

func waitDuration(ctx context.Context, duration time.Duration) error {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}
