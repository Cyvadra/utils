package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type sendMessageRequest struct {
	ChatID    string `json:"chat_id"`
	Text      string `json:"text"`
	ParseMode string `json:"parse_mode,omitempty"`
}

type telegramResponse struct {
	OK     bool `json:"ok"`
	Result struct {
		ID   int64 `json:"message_id"`
		Date int64 `json:"date"`
		Chat struct {
			ID json.RawMessage `json:"id"`
		} `json:"chat"`
		Text string `json:"text"`
	} `json:"result"`
	ErrorCode   int    `json:"error_code"`
	Description string `json:"description"`
	Parameters  struct {
		RetryAfter      int64           `json:"retry_after"`
		MigrateToChatID json.RawMessage `json:"migrate_to_chat_id"`
	} `json:"parameters"`
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
	for attempt := 0; attempt < 2; attempt++ {
		if err := service.wait(ctx); err != nil {
			return Message{}, err
		}
		message, err := service.dispatch(ctx, payload)
		if err == nil {
			return message, nil
		}
		var apiError *APIError
		if attempt == 0 && errors.As(err, &apiError) && apiError.Code == http.StatusTooManyRequests && apiError.RetryAfter > 0 {
			if err := waitDuration(ctx, apiError.RetryAfter); err != nil {
				return Message{}, err
			}
			continue
		}
		return Message{}, err
	}
	return Message{}, errors.New("telegram: retry attempts exhausted")
}

func (service *Service) wait(ctx context.Context) error {
	if err := service.botLimiter.wait(ctx); err != nil {
		return err
	}
	return service.chatLimiter.wait(ctx)
}

func (service *Service) dispatch(ctx context.Context, payload sendMessageRequest) (Message, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return Message{}, fmt.Errorf("telegram: encode message: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, service.baseURL+"/bot"+service.token+"/sendMessage", bytes.NewReader(body))
	if err != nil {
		return Message{}, fmt.Errorf("telegram: create request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := service.client.Do(request)
	if err != nil {
		return Message{}, fmt.Errorf("telegram: send request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		io.Copy(io.Discard, response.Body)
		return Message{}, &HTTPStatusError{StatusCode: response.StatusCode}
	}

	var result telegramResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return Message{}, fmt.Errorf("%w: %v", ErrInvalidResponse, err)
	}
	if !result.OK {
		return Message{}, &APIError{
			Code:            result.ErrorCode,
			Description:     result.Description,
			RetryAfter:      time.Duration(result.Parameters.RetryAfter) * time.Second,
			MigrateToChatID: chatIDFromJSON(result.Parameters.MigrateToChatID),
		}
	}
	return Message{
		ID:     result.Result.ID,
		Date:   time.Unix(result.Result.Date, 0),
		ChatID: chatIDFromJSON(result.Result.Chat.ID),
		Text:   result.Result.Text,
	}, nil
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
