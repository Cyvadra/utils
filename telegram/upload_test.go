package telegram

import (
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/Cyvadra/utils/internal/testutil"
	"github.com/stretchr/testify/require"
)

type uploadForm struct {
	fields map[string]string
	files  map[string]string
}

func parseUpload(t *testing.T, recorded testutil.RecordedRequest) uploadForm {
	t.Helper()
	mediaType, params, err := mime.ParseMediaType(recorded.ContentType)
	require.NoError(t, err)
	require.Equal(t, "multipart/form-data", mediaType)
	form := uploadForm{fields: map[string]string{}, files: map[string]string{}}
	reader := multipart.NewReader(strings.NewReader(recorded.Body), params["boundary"])
	for {
		part, err := reader.NextPart()
		if err == io.EOF {
			return form
		}
		require.NoError(t, err)
		data, err := io.ReadAll(part)
		require.NoError(t, err)
		if part.FileName() != "" {
			form.files[part.FormName()] = part.FileName() + ":" + string(data)
		} else {
			form.fields[part.FormName()] = string(data)
		}
	}
}

func groupResponse(n int) *http.Response {
	items := make([]string, n)
	for index := range items {
		items[index] = `{"message_id":` + strconv.Itoa(index+1) + `,"date":1700000000,"chat":{"id":42},"caption":"c"}`
	}
	return testutil.Response(http.StatusOK, `{"ok":true,"result":[`+strings.Join(items, ",")+`]}`)
}

func TestSendPhotosSingle(t *testing.T) {
	resetServices(t)
	transport := newTransport(testutil.Response(http.StatusOK,
		`{"ok":true,"result":{"message_id":7,"date":1700000000,"chat":{"id":42},"caption":"wallet"}}`))
	service, err := Register("photos", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)

	messages, err := service.SendPhotos(context.Background(), File{Name: "wallet.png", Data: []byte("png"), Caption: "wallet"})
	require.NoError(t, err)
	require.Equal(t, []Message{{ID: 7, Date: messages[0].Date, ChatID: "42", Text: "wallet"}}, messages)
	require.Len(t, transport.Requests, 1)
	require.Equal(t, "https://api.test/bot"+testToken+"/sendPhoto", transport.Requests[0].URL)
	form := parseUpload(t, transport.Requests[0])
	require.Equal(t, map[string]string{"chat_id": "42", "caption": "wallet"}, form.fields)
	require.Equal(t, map[string]string{"photo": "wallet.png:png"}, form.files)
}

func TestSendFilesChunksIntoMediaGroups(t *testing.T) {
	resetServices(t)
	transport := newTransport(groupResponse(MediaGroupLimit))
	service, err := Register("files", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)

	files := make([]File, MediaGroupLimit+1)
	for index := range files {
		files[index] = File{Data: []byte{byte('a' + index)}, Caption: string(rune('A' + index))}
	}
	messages, err := service.SendFiles(context.Background(), files...)
	require.NoError(t, err)
	require.Len(t, messages, MediaGroupLimit+1)
	require.Len(t, transport.Requests, 2)

	require.Equal(t, "https://api.test/bot"+testToken+"/sendMediaGroup", transport.Requests[0].URL)
	group := parseUpload(t, transport.Requests[0])
	require.Equal(t, "42", group.fields["chat_id"])
	var media []map[string]string
	require.NoError(t, json.Unmarshal([]byte(group.fields["media"]), &media))
	require.Len(t, media, MediaGroupLimit)
	require.Equal(t, map[string]string{"type": "document", "media": "attach://file0", "caption": "A"}, media[0])
	require.Equal(t, "file1:a", group.files["file0"])
	require.Equal(t, "file10:j", group.files["file9"])

	// The leftover single file cannot form a media group.
	require.Equal(t, "https://api.test/bot"+testToken+"/sendDocument", transport.Requests[1].URL)
	single := parseUpload(t, transport.Requests[1])
	require.Equal(t, "K", single.fields["caption"])
	require.Equal(t, map[string]string{"document": "file11:k"}, single.files)
}

func TestSendPhotosRejectsEmpty(t *testing.T) {
	resetServices(t)
	transport := newTransport()
	service, err := Register("empty", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)
	_, err = service.SendPhotos(context.Background())
	require.ErrorIs(t, err, ErrNoFiles)
	_, err = service.SendPhotos(context.Background(), File{Name: "x.png"})
	require.Error(t, err)
	require.Empty(t, transport.Requests)
}

func TestSendPhotosAPIError(t *testing.T) {
	resetServices(t)
	transport := newTransport(testutil.Response(http.StatusOK, `{"ok":false,"error_code":400,"description":"Bad Request: wrong file"}`))
	service, err := Register("bad", testToken, "42", testOptions(transport)...)
	require.NoError(t, err)
	_, err = service.SendPhotos(context.Background(), File{Data: []byte("x")}, File{Data: []byte("y")})
	var apiError *APIError
	require.ErrorAs(t, err, &apiError)
	require.Equal(t, 400, apiError.Code)
}
