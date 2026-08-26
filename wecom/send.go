package wecom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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

// Send sends a text message. A provided Mention applies to every chunk of a long message.
func (service *Service) Send(ctx context.Context, content string, mentions ...Mention) error {
	mention := mergeMentions(mentions)
	return service.sendText(ctx, content, mention)
}

// SendMarkdown sends a classic WeCom markdown message. To mention a user, use the
// supported inline <@userid> syntax in content.
func (service *Service) SendMarkdown(ctx context.Context, content string) error {
	chunks := splitContent(content, MarkdownLimit)
	for index, chunk := range chunks {
		payload := markdownPayload{MessageType: "markdown"}
		payload.Markdown.Content = chunk
		if err := service.dispatch(ctx, payload); err != nil {
			return fmt.Errorf("wecom: send markdown chunk %d of %d: %w", index+1, len(chunks), err)
		}
	}
	return nil
}

func (service *Service) sendText(ctx context.Context, content string, mention Mention) error {
	chunks := splitContent(content, TextLimit)
	for index, chunk := range chunks {
		payload := textPayload{MessageType: "text"}
		payload.Text.Content = chunk
		payload.Text.MentionedList = mention.UserIDs
		payload.Text.MentionedMobileList = mention.MobileNumbers
		if err := service.dispatch(ctx, payload); err != nil {
			return fmt.Errorf("wecom: send text chunk %d of %d: %w", index+1, len(chunks), err)
		}
	}
	return nil
}

func (service *Service) dispatch(ctx context.Context, payload any) error {
	if err := service.limiter.wait(ctx); err != nil {
		return err
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("wecom: encode message: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, service.webhookURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("wecom: create webhook request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")

	response, err := service.client.Do(request)
	if err != nil {
		return fmt.Errorf("wecom: send webhook request: %w", err)
	}
	defer response.Body.Close()
	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		io.Copy(io.Discard, response.Body)
		return &HTTPStatusError{StatusCode: response.StatusCode}
	}

	var result webhookResponse
	if err := json.NewDecoder(response.Body).Decode(&result); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidResponse, err)
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
	return merged
}
