package web

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"

	s3types "github.com/aws/aws-sdk-go-v2/service/s3/types"
	"github.com/h2non/filetype"
	"github.com/nyaruka/chip/v26/core/models"
	"github.com/nyaruka/gocommon/uuids"
	"github.com/nyaruka/vkutil"
)

const (
	// the most a visitor can upload in one file - matching what the platform accepts for media uploads
	// elsewhere - and how many uploads a single chat can make per window
	maxUploadBytes    = 25 * 1024 * 1024
	uploadLimit       = 5
	uploadLimitWindow = time.Minute

	// how many uploads a single IP can make on a channel per window - a backstop above the per-chat limit (so
	// visitors sharing a NAT aren't broken) that a caller can't reset by rotating chat IDs. It also bounds upload
	// memory per source: the worst case is this many bodies buffered per IP per window.
	uploadIPLimit       = 10
	uploadIPLimitWindow = time.Minute

	// slack on the request body cap for the multipart framing and fields around the file itself
	uploadFormOverheadBytes = 64 * 1024
)

var (
	uploadLimiter   = vkutil.NewRateLimiter("chat-uploads", uploadLimit, uploadLimitWindow)
	uploadIPLimiter = vkutil.NewRateLimiter("chat-uploads-ip", uploadIPLimit, uploadIPLimitWindow)
)

// the media types a visitor may upload - the same kinds of media the platform handles on other channels. Entries
// ending in / allow a whole family of types.
var allowedUploadTypes = []string{"image/", "audio/", "video/", "application/pdf"}

// uploadTypeAllowed returns whether the given media type is one visitors may upload
func uploadTypeAllowed(mime string) bool {
	return mime != "" && slices.ContainsFunc(allowedUploadTypes, func(a string) bool {
		return mime == a || (strings.HasSuffix(a, "/") && strings.HasPrefix(mime, a))
	})
}

// uploadURLPrefix is what the storage URLs of this channel's uploaded attachments all start with - which scopes a
// message's attachment references to files uploaded for this channel's workspace
func (s *Server) uploadURLPrefix(ch *models.Channel) string {
	return s.rt.S3.ObjectURL(s.rt.Config.S3AttachmentsBucket, fmt.Sprintf("attachments/%d/", ch.OrgID()))
}

// saveAttachment saves an uploaded file to the workspace's attachment storage, under the same layout as the
// platform's other attachments, and returns its URL
func (s *Server) saveAttachment(ctx context.Context, ch *models.Channel, contentType string, data []byte, extension string) (string, error) {
	filename := string(uuids.NewV4())
	if extension != "" {
		filename = fmt.Sprintf("%s.%s", filename, extension)
	}

	key := path.Join("attachments", strconv.Itoa(int(ch.OrgID())), filename[:4], filename[4:8], filename)

	url, err := s.rt.S3.PutObject(ctx, s.rt.Config.S3AttachmentsBucket, key, contentType, data, s3types.ObjectCannedACLPublicRead)
	if err != nil {
		return "", fmt.Errorf("error saving attachment to storage (bytes=%d): %w", len(data), err)
	}
	return url, nil
}

type uploadResponse struct {
	Attachment string `json:"attachment"`
}

// handleUpload is our handler for a visitor adding a file to their chat: the file is saved to attachment storage
// and the visitor gets back its content-type:url attachment value, to be referenced in a subsequent message to the
// receive endpoint. Like receive, possession of the chat ID is what authenticates the caller.
func (s *Server) handleUpload(ctx context.Context, ch *models.Channel, w http.ResponseWriter, r *http.Request) error {
	// the per-chat throttle below can't run until the form is parsed, so on its own it lets a caller buffer bodies
	// freely by rotating chat IDs. This per-IP cap doesn't need the body, so it runs first and a throttled request
	// is rejected before anything is buffered. Like start, distributed floods are left to edge protection.
	if !s.allow(ctx, uploadIPLimiter, fmt.Sprintf("%s|%s", ch.UUID(), requestIP(r))) {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return nil
	}

	// cap the body before parsing so an oversize upload is cut off rather than buffered in full
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes+uploadFormOverheadBytes)

	// the whole cap fits in memory, so nothing spills to temp files
	if err := r.ParseMultipartForm(maxUploadBytes + uploadFormOverheadBytes); err != nil {
		var maxErr *http.MaxBytesError
		if errors.As(err, &maxErr) {
			writeError(w, http.StatusRequestEntityTooLarge, fmt.Sprintf("upload too large (max %d bytes)", maxUploadBytes))
			return nil
		}
		writeError(w, http.StatusBadRequest, "invalid multipart form")
		return nil
	}

	chatID := r.FormValue("chat_id")

	// validated before the throttle so malformed chat IDs can't mint valkey keys or share one empty-ID key
	if _, err := models.ChatURN(chatID); err != nil {
		writeError(w, http.StatusBadRequest, "invalid chat id")
		return nil
	}

	// throttled per chat rather than per IP - a visitor attaching files only needs a handful at a time
	if !s.allow(ctx, uploadLimiter, fmt.Sprintf("%s|%s", ch.UUID(), chatID)) {
		writeError(w, http.StatusTooManyRequests, "rate limit exceeded")
		return nil
	}

	visitor, err := models.LoadVisitor(ctx, s.rt.DB, ch, chatID)
	if err != nil {
		return err
	}
	if visitor == nil {
		writeError(w, http.StatusBadRequest, "unknown chat id")
		return nil
	}

	file, _, err := r.FormFile("file")
	if err != nil {
		writeError(w, http.StatusBadRequest, "missing file part")
		return nil
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return fmt.Errorf("error reading uploaded file: %w", err)
	}

	// the content type comes from sniffing the bytes rather than trusting the client's declared type - an
	// unrecognized file is rejected along with recognized-but-disallowed types
	fileType, _ := filetype.Match(data)
	if fileType == filetype.Unknown || !uploadTypeAllowed(fileType.MIME.Value) {
		writeError(w, http.StatusBadRequest, "unsupported file type")
		return nil
	}

	storageURL, err := s.saveAttachment(ctx, ch, fileType.MIME.Value, data, fileType.Extension)
	if err != nil {
		return err
	}

	writeJSON(w, http.StatusOK, &uploadResponse{Attachment: fmt.Sprintf("%s:%s", fileType.MIME.Value, storageURL)})
	return nil
}
