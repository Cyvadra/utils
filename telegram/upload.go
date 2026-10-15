package telegram

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"strconv"
)

// MediaGroupLimit is Telegram's maximum number of items in one media group.
const MediaGroupLimit = 10

var singleMethods = map[string]string{"photo": "sendPhoto", "document": "sendDocument"}

var ErrNoFiles = errors.New("telegram: no files to send")

// File is one upload. Caption is optional.
type File struct {
	Name    string
	Data    []byte
	Caption string
}

// SendPhotos sends images to the service's default chat. One photo is sent with
// sendPhoto; several are sent in order as albums of up to MediaGroupLimit.
func (service *Service) SendPhotos(ctx context.Context, photos ...File) ([]Message, error) {
	return service.sendFiles(ctx, "photo", photos)
}

// SendFiles sends documents to the service's default chat. One file is sent
// with sendDocument; several are sent in order as albums of up to MediaGroupLimit.
func (service *Service) SendFiles(ctx context.Context, files ...File) ([]Message, error) {
	return service.sendFiles(ctx, "document", files)
}

func (service *Service) sendFiles(ctx context.Context, kind string, files []File) ([]Message, error) {
	if len(files) == 0 {
		return nil, ErrNoFiles
	}
	for index, file := range files {
		if len(file.Data) == 0 {
			return nil, fmt.Errorf("telegram: %s %d is empty", kind, index+1)
		}
	}
	messages := make([]Message, 0, len(files))
	for start := 0; start < len(files); start += MediaGroupLimit {
		group := files[start:min(start+MediaGroupLimit, len(files))]
		sent, err := service.sendGroup(ctx, kind, group, start)
		messages = append(messages, sent...)
		if err != nil {
			return messages, fmt.Errorf("telegram: send %s %d of %d: %w", kind, start+1, len(files), err)
		}
	}
	return messages, nil
}

// sendGroup sends one request; a group of one uses the single-file method,
// since Telegram rejects media groups with fewer than two items.
func (service *Service) sendGroup(ctx context.Context, kind string, group []File, offset int) ([]Message, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	fields := map[string]string{"chat_id": service.chatID}
	method := "sendMediaGroup"
	if len(group) == 1 {
		method = singleMethods[kind]
		if group[0].Caption != "" {
			fields["caption"] = group[0].Caption
		}
		if err := writeFile(writer, kind, group[0], offset); err != nil {
			return nil, err
		}
	} else {
		type inputMedia struct {
			Type    string `json:"type"`
			Media   string `json:"media"`
			Caption string `json:"caption,omitempty"`
		}
		media := make([]inputMedia, len(group))
		for index, file := range group {
			part := "file" + strconv.Itoa(index)
			media[index] = inputMedia{Type: kind, Media: "attach://" + part, Caption: file.Caption}
			if err := writeFile(writer, part, file, offset+index); err != nil {
				return nil, err
			}
		}
		encoded, err := json.Marshal(media)
		if err != nil {
			return nil, fmt.Errorf("encode media: %w", err)
		}
		fields["media"] = string(encoded)
	}
	for name, value := range fields {
		if err := writer.WriteField(name, value); err != nil {
			return nil, err
		}
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	req := request{method: method, contentType: writer.FormDataContentType(), body: body.Bytes()}
	if len(group) == 1 {
		var result messageResult
		if err := service.call(ctx, req, &result); err != nil {
			return nil, err
		}
		return []Message{result.message()}, nil
	}
	var results []messageResult
	if err := service.call(ctx, req, &results); err != nil {
		return nil, err
	}
	messages := make([]Message, len(results))
	for index, result := range results {
		messages[index] = result.message()
	}
	return messages, nil
}

func writeFile(writer *multipart.Writer, field string, file File, index int) error {
	name := file.Name
	if name == "" {
		name = "file" + strconv.Itoa(index+1)
	}
	part, err := writer.CreateFormFile(field, name)
	if err != nil {
		return err
	}
	_, err = part.Write(file.Data)
	return err
}
