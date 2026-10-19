// Package testsuite sets up what a test needs: a runtime on its own database and valkey, with courier and the
// realtime server mocked.
package testsuite

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path"
	goruntime "runtime"
	"sync"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/nyaruka/chip/v26/runtime"
	"github.com/nyaruka/gocommon/centrifugo"
	"github.com/nyaruka/vkutil/assertvk"
	"github.com/stretchr/testify/require"
)

// testdataPath returns the absolute path of a file in this package's testdata directory. Paths are resolved relative
// to this source file rather than the module being tested, so that they also work from modules importing this package.
func testdataPath(file string) string {
	_, thisFile, _, _ := goruntime.Caller(0)
	return path.Join(path.Dir(thisFile), "testdata", file)
}

// random per-binary identifier used in per-binary resource names. Can't use the pid because binaries run in
// per-worktree containers which share the backing services, so pids are neither unique nor observable across them.
var binProcID = sync.OnceValue(func() string {
	b := make([]byte, 4)
	rand.Read(b)
	return hex.EncodeToString(b)
})

// Runtime returns the various runtime things a test might need
func Runtime(t *testing.T) (context.Context, *runtime.Runtime) {
	// each test gets its own database cloned from a template built from our dump - see dbtemplate.go
	dbName := createTestDB(t)
	t.Cleanup(func() { dropTestDB(t, dbName) })

	// and its own valkey database - see valkey.go
	vk := assertvk.ClaimDB(t)

	cfg := runtime.NewDefaultConfig()
	cfg.DeploymentID = "test"
	cfg.DB = fmt.Sprintf(dbTestDSNFormat, dbName)
	cfg.Valkey = vk.URL
	cfg.AuthToken = "sesame"
	cfg.CourierAuthToken = "opensesame"
	cfg.CentrifugoEndpoint = "http://centrifugo.test/ws/api" // never called - the client is mocked below

	// AWS SDK default chain reads these - used by the S3 client
	t.Setenv("AWS_ACCESS_KEY_ID", "root")
	t.Setenv("AWS_SECRET_ACCESS_KEY", "tembatemba")
	t.Setenv("AWS_REGION", "us-east-1")

	// the dev stack's S3 (SeaweedFS), with test buckets of our own
	cfg.S3Endpoint = "http://s3:8333"
	cfg.S3AttachmentsBucket = "test-attachments"
	cfg.S3PublicBucket = "test-default"
	cfg.S3PathStyle = true

	require.NoError(t, cfg.Parse())

	rt, err := runtime.NewRuntime(cfg)
	require.NoError(t, err)

	rt.Centrifugo = centrifugo.NewService(centrifugo.NewMockClient(), rt.VK)

	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelWarn})))

	// uploads are saved to attachment storage, so ensure the bucket exists
	rt.S3.Client.CreateBucket(t.Context(), &s3.CreateBucketInput{Bucket: aws.String(cfg.S3AttachmentsBucket)})

	t.Cleanup(func() { rt.Stop() })

	return t.Context(), rt
}

// Publications returns the data of everything published to the given socket, oldest first
func Publications(t *testing.T, rt *runtime.Runtime, socket string) []json.RawMessage {
	t.Helper()

	var pubs []json.RawMessage
	for _, p := range rt.Centrifugo.Client.(*centrifugo.MockClient).Publications() {
		if p.Channel == socket {
			pubs = append(pubs, p.Data.(json.RawMessage))
		}
	}
	return pubs
}
