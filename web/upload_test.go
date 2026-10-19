package web

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/chip/v26/testsuite/testdb"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// upload posts a multipart upload request from the given IP - a nil file omits the file part entirely
func upload(t *testing.T, s *Server, chatID, ip string, file []byte) *response {
	t.Helper()

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	require.NoError(t, mw.WriteField("chat_id", chatID))
	if file != nil {
		fw, err := mw.CreateFormFile("file", "upload.bin")
		require.NoError(t, err)
		_, err = fw.Write(file)
		require.NoError(t, err)
	}
	require.NoError(t, mw.Close())

	req := httptest.NewRequest(http.MethodPost, "https://localhost"+publicPrefix+"/"+string(channelUUID)+"/upload", bytes.NewReader(body.Bytes()))
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.RemoteAddr = ip
	return request(s.PublicHandler(), req)
}

func testJPG(t *testing.T) []byte {
	t.Helper()

	d, err := os.ReadFile("../testsuite/testdata/test.jpg")
	require.NoError(t, err)
	return d
}

func TestUpload(t *testing.T) {
	rt, s, mock := setup(t)
	ch, _ := models.LoadChannel(t.Context(), rt.DB, channelUUID)
	testdb.InsertVisitor(t, rt, ch, testChatID)

	jpg := testJPG(t)

	// a valid upload returns the stored attachment as a content-type:url value, typed by sniffing the bytes and
	// stored under this channel's workspace
	resp := upload(t, s, testChatID, "41.23.45.66:1234", jpg)
	assert.Equal(t, 200, resp.Code, resp.Body)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	r := &uploadResponse{}
	resp.JSON(t, r)
	prefix := "image/jpeg:http://s3:8333/test-attachments/attachments/1/"
	assert.True(t, strings.HasPrefix(r.Attachment, prefix), r.Attachment)
	assert.True(t, strings.HasSuffix(r.Attachment, ".jpg"), r.Attachment)

	// whose URL serves back exactly what was uploaded
	storageURL := strings.TrimPrefix(r.Attachment, "image/jpeg:")
	key := strings.TrimPrefix(storageURL, "http://s3:8333/"+rt.Config.S3AttachmentsBucket+"/")
	contentType, body, err := rt.S3.GetObject(t.Context(), rt.Config.S3AttachmentsBucket, key)
	require.NoError(t, err)
	assert.Equal(t, "image/jpeg", contentType)
	assert.Equal(t, jpg, body)

	// and whose attachment value the receive endpoint accepts on a message with no text
	resp = post(s, channelUUID, "receive", `{"chat_id": "`+testChatID+`", "attachments": ["`+r.Attachment+`"]}`)
	assert.Equal(t, 200, resp.Code, resp.Body)
	assert.Contains(t, string(mock.Requests()[0].Body), r.Attachment)

	// a recognized but disallowed file type is rejected, with the CORS header still on the error so the widget
	// can read it
	zip := append([]byte{0x50, 0x4b, 0x03, 0x04}, make([]byte, 100)...)
	resp = upload(t, s, testChatID, "41.23.45.66:1234", zip)
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "unsupported file type"}`, resp.Body)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	// as is a file whose type can't be recognized at all
	resp = upload(t, s, testChatID, "41.23.45.66:1234", []byte("just some text"))
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "unsupported file type"}`, resp.Body)

	// an oversize upload is cut off rather than buffered
	resp = upload(t, s, testChatID, "41.23.45.66:1234", make([]byte, maxUploadBytes+uploadFormOverheadBytes+1))
	assert.Equal(t, 413, resp.Code)
	assert.JSONEq(t, fmt.Sprintf(`{"error": "upload too large (max %d bytes)"}`, maxUploadBytes), resp.Body)

	// a chat ID we never minted is a bad request, as is a malformed one or a request with no file part
	resp = upload(t, s, "xxxxxhDrqpTQefIEinK0up3C", "41.23.45.66:1234", jpg)
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "unknown chat id"}`, resp.Body)

	resp = upload(t, s, "not-a-chat-id!", "41.23.45.66:1234", jpg)
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "invalid chat id"}`, resp.Body)

	resp = upload(t, s, testChatID, "41.23.45.66:1234", nil)
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "missing file part"}`, resp.Body)

	// as is something that isn't a multipart form at all
	resp = post(s, channelUUID, "upload", `{}`)
	assert.Equal(t, 400, resp.Code)
	assert.JSONEq(t, `{"error": "invalid multipart form"}`, resp.Body)
}

func TestUploadRateLimit(t *testing.T) {
	rt, s, _ := setup(t)
	ch, _ := models.LoadChannel(t.Context(), rt.DB, channelUUID)
	testdb.InsertVisitor(t, rt, ch, testChatID)

	jpg := testJPG(t)

	// a chat can upload up to the limit of files within the window...
	for i := range uploadLimit {
		assert.Equal(t, 200, upload(t, s, testChatID, "41.23.45.67:1234", jpg).Code, "upload %d", i)
	}

	// ...then gets throttled, with the CORS header still on the error so the widget can read it
	resp := upload(t, s, testChatID, "41.23.45.67:1234", jpg)
	assert.Equal(t, 429, resp.Code)
	assert.JSONEq(t, `{"error": "rate limit exceeded"}`, resp.Body)
	assert.Equal(t, "*", resp.Header.Get("Access-Control-Allow-Origin"))

	// the per-chat limit follows the chat, not the caller's IP
	assert.Equal(t, 429, upload(t, s, testChatID, "41.23.45.68:1234", jpg).Code)

	// but other chats aren't affected - the limit is per chat (an unknown one just fails its lookup)
	assert.Equal(t, 400, upload(t, s, "xxxxxhDrqpTQefIEinK0up3C", "41.23.45.67:1234", jpg).Code)

	// rotating chat IDs doesn't evade limiting - the per-IP cap counts every request from a source however it
	// labels itself
	for i := range uploadIPLimit {
		chatID := fmt.Sprintf("xxxxxhDrqpTQefIEinK0u%03d", i) // format-valid but never minted
		assert.Equal(t, 400, upload(t, s, chatID, "41.23.45.69:1234", jpg).Code, "rotation %d", i)
	}
	resp = upload(t, s, "xxxxxhDrqpTQefIEinK0u999", "41.23.45.69:1234", jpg)
	assert.Equal(t, 429, resp.Code)
	assert.JSONEq(t, `{"error": "rate limit exceeded"}`, resp.Body)

	// and both counts expire with their windows
	assertTTL(t, rt, "chat-uploads:"+string(channelUUID)+"|"+testChatID, uploadLimitWindow)
	assertTTL(t, rt, "chat-uploads-ip:"+string(channelUUID)+"|41.23.45.69", uploadIPLimitWindow)
}

func TestUploadTypeAllowed(t *testing.T) {
	for mime, allowed := range map[string]bool{
		"image/jpeg": true, "image/png": true, "audio/mpeg": true, "video/mp4": true, "application/pdf": true,
		"application/zip": false, "text/plain": false, "image": false, "": false,
	} {
		assert.Equal(t, allowed, uploadTypeAllowed(mime), mime)
	}
}
