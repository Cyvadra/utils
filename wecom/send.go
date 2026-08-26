package wecom

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/Cyvadra/utils/internal/webhook"
)

type webhookResponse struct {
	ErrorCode int    `json:"errcode"`
	ErrorMsg  string `json:"errmsg"`
}

type textPayload struct {
	MessageType string `json:"msgtype"`
	Text        struct {
		Content             string   `json:"content"`
		MentionedList       []string `json:"mentioned_list,omitempty"`
		MentionedMobileList []string `json:"mentioned_mobile_list,omitempty"`
	} `json:"text"`
}

type markdownPayload struct {
	MessageType string `json:"msgtype"`
	Markdown    struct {
		Content string `json:"content"`
	} `json:"markdown"`
}

// Send sends a text message and returns the number of chunks delivered. A
// provided Mention applies to the first chunk of a long message.
func (service *Service) Send(ctx context.Context, content string, mentions ...Mention) (int, error) {
	if strings.TrimSpace(content) == "" {
		return 0, ErrEmptyContent
	}
	return service.sendText(ctx, content, mergeMentions(mentions))
}

// SendMarkdown sends a classic WeCom markdown message and returns the number of
// chunks delivered. To mention a user, use the supported inline <@userid> syntax.
func (service *Service) SendMarkdown(ctx context.Context, content string) (int, error) {
	if strings.TrimSpace(content) == "" {
		return 0, ErrEmptyContent
	}
	chunks := splitContent(content, MarkdownLimit)
	for index, chunk := range chunks {
		payload := markdownPayload{MessageType: "markdown"}
		payload.Markdown.Content = chunk
		if err := service.dispatch(ctx, payload); err != nil {
			return index, fmt.Errorf("wecom: send markdown chunk %d of %d: %w", index+1, len(chunks), err)
		}
	}
	return len(chunks), nil
}

func (service *Service) sendText(ctx context.Context, content string, mention Mention) (int, error) {
	chunks := splitContent(content, TextLimit)
	for index, chunk := range chunks {
		payload := textPayload{MessageType: "text"}
		payload.Text.Content = chunk
		if index == 0 {
			payload.Text.MentionedList = mention.UserIDs
			payload.Text.MentionedMobileList = mention.MobileNumbers
		}
		if err := service.dispatch(ctx, payload); err != nil {
			return index, fmt.Errorf("wecom: send text chunk %d of %d: %w", index+1, len(chunks), err)
		}
	}
	return len(chunks), nil
}

func (service *Service) dispatch(ctx context.Context, payload any) error {
	if err := service.limiter.Wait(ctx); err != nil {
		return err
	}
	var result webhookResponse
	if err := webhook.SendJSON(ctx, service.client, service.webhookURL, payload, &result); err != nil {
		var statusErr *webhook.HTTPStatusError
		if errors.As(err, &statusErr) {
			return &HTTPStatusError{StatusCode: statusErr.StatusCode}
		}
		var decodeErr *webhook.DecodeError
		if errors.As(err, &decodeErr) {
			return fmt.Errorf("%w: %v", ErrInvalidResponse, decodeErr.Err)
		}
		return fmt.Errorf("wecom: %w", err)
	}
	if result.ErrorCode != 0 {
		return &APIError{Code: result.ErrorCode, Message: result.ErrorMsg}
	}
	return nil
}

func mergeMentions(mentions []Mention) Mention {
	var merged Mention
	for _, mention := range mentions {
		merged.UserIDs = append(merged.UserIDs, mention.UserIDs...)
		merged.MobileNumbers = append(merged.MobileNumbers, mention.MobileNumbers...)
		merged.All = merged.All || mention.All
	}
	if merged.All {
		merged.UserIDs = append(merged.UserIDs, "@all")
	}
	merged.UserIDs = uniqueStrings(merged.UserIDs)
	merged.MobileNumbers = uniqueStrings(merged.MobileNumbers)
	return merged
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	result := make([]string, 0, len(values))
	for _, value := range values {
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}
