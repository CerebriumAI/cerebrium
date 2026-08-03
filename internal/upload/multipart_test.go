package upload

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/url"
	"sync"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/neterr"
)

const presignedURL = "https://bucket.s3.us-east-1.amazonaws.com/key.zip?X-Amz-Signature=deadbeefcafe"

// fakeSession records the initiate/complete calls and can fail either one.
type fakeSession struct {
	partCount    int
	uploadID     string
	initiateErr  error
	completeErr  error
	completedID  string
	completed    []api.PartInfo
	initiateCall int
}

func (f *fakeSession) Initiate(_ context.Context, partCount int) (*api.InitiateUploadResponse, error) {
	f.initiateCall++
	f.partCount = partCount
	if f.initiateErr != nil {
		return nil, f.initiateErr
	}

	parts := make([]api.PartURL, 0, partCount)
	for i := 1; i <= partCount; i++ {
		parts = append(parts, api.PartURL{PartNumber: i, URL: partURL(i)})
	}
	return &api.InitiateUploadResponse{UploadID: f.uploadID, Parts: parts}, nil
}

func (f *fakeSession) Complete(_ context.Context, uploadID string, parts []api.PartInfo) error {
	f.completedID = uploadID
	f.completed = parts
	return f.completeErr
}

// fakeUploader records the bytes seen per part and can be scripted to fail.
type fakeUploader struct {
	mu       sync.Mutex
	received map[string][]byte
	attempts map[string]int
	inFlight atomic.Int32
	maxSeen  atomic.Int32

	// failFor maps a part URL to a function returning the error for attempt n.
	failFor func(url string, attempt int) error
	delay   time.Duration
}

func newFakeUploader() *fakeUploader {
	return &fakeUploader{
		received: map[string][]byte{},
		attempts: map[string]int{},
	}
}

func (f *fakeUploader) UploadPart(_ context.Context, partURL string, data []byte) (string, error) {
	cur := f.inFlight.Add(1)
	for {
		max := f.maxSeen.Load()
		if cur <= max || f.maxSeen.CompareAndSwap(max, cur) {
			break
		}
	}
	defer f.inFlight.Add(-1)

	if f.delay > 0 {
		time.Sleep(f.delay)
	}

	f.mu.Lock()
	f.attempts[partURL]++
	attempt := f.attempts[partURL]
	f.mu.Unlock()

	if f.failFor != nil {
		if err := f.failFor(partURL, attempt); err != nil {
			return "", err
		}
	}

	f.mu.Lock()
	f.received[partURL] = append([]byte(nil), data...)
	f.mu.Unlock()

	return fmt.Sprintf(`"etag-%s"`, partURL), nil
}

func (f *fakeUploader) attemptsFor(partNumber int) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.attempts[partURL(partNumber)]
}

// testPartSize keeps the arithmetic in these tests easy to read.
const testPartSize = 100

// partURL is the presigned URL fakeSession hands out for a given part.
func partURL(partNumber int) string {
	return fmt.Sprintf("https://bucket.s3.amazonaws.com/key?partNumber=%d&X-Amz-Signature=sig%d", partNumber, partNumber)
}

// fastOpts keeps retry backoff negligible so tests stay quick.
func fastOpts() Options {
	return Options{
		PartSize:    testPartSize,
		Concurrency: 4,
		MaxAttempts: 3,
		RetryDelay:  time.Millisecond,
		Op:          "upload build artifact",
	}
}

func TestPartCount(t *testing.T) {
	tests := []struct {
		name     string
		size     int64
		partSize int64
		want     int
	}{
		{name: "empty file still needs one part", size: 0, partSize: 100, want: 1},
		{name: "exactly one part", size: 100, partSize: 100, want: 1},
		{name: "one byte over", size: 101, partSize: 100, want: 2},
		{name: "exact multiple", size: 500, partSize: 100, want: 5},
		{name: "partial last part", size: 450, partSize: 100, want: 5},
		{name: "zero part size falls back to default", size: DefaultPartSize + 1, partSize: 0, want: 2},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, PartCount(tt.size, tt.partSize))
		})
	}
}

func TestDoUploadsEveryByteExactlyOnce(t *testing.T) {
	// 450 bytes over a 100-byte part size: 4 full parts and a 50-byte tail.
	payload := bytes.Repeat([]byte("abcdefghij"), 45)
	require.Len(t, payload, 450)

	sess := &fakeSession{uploadID: "upload-1"}
	uploader := newFakeUploader()
	progress := &atomic.Int64{}

	opts := fastOpts()
	opts.Progress = progress

	err := Do(t.Context(), bytes.NewReader(payload), int64(len(payload)), sess, uploader, opts)
	require.NoError(t, err)

	assert.Equal(t, 5, sess.partCount)
	assert.Equal(t, "upload-1", sess.completedID)
	require.Len(t, sess.completed, 5)

	// Parts are reported in order, with the ETag the uploader returned.
	var reassembled []byte
	for i, part := range sess.completed {
		assert.Equal(t, i+1, part.PartNumber)
		assert.NotEmpty(t, part.ETag)

		reassembled = append(reassembled, uploader.received[partURL(i+1)]...)
	}

	// The concatenated parts are byte-identical to the input.
	assert.Equal(t, payload, reassembled)
	assert.Equal(t, int64(len(payload)), progress.Load())
}

func TestDoLastPartIsNotPadded(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 250)
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()

	require.NoError(t, Do(t.Context(), bytes.NewReader(payload), int64(len(payload)), sess, uploader, fastOpts()))

	lastURL := partURL(3)
	assert.Len(t, uploader.received[lastURL], 50, "tail part must not be zero-padded to the full part size")
}

func TestDoEmptyFile(t *testing.T) {
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()

	require.NoError(t, Do(t.Context(), bytes.NewReader(nil), 0, sess, uploader, fastOpts()))

	assert.Equal(t, 1, sess.partCount)
	require.Len(t, sess.completed, 1)
}

func TestDoRespectsConcurrencyLimit(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 1000)
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()
	uploader.delay = 5 * time.Millisecond

	opts := fastOpts() // 10 parts
	opts.Concurrency = 3

	require.NoError(t, Do(t.Context(), bytes.NewReader(payload), int64(len(payload)), sess, uploader, opts))

	assert.LessOrEqual(t, int(uploader.maxSeen.Load()), 3)
}

// TestDoDoesNotRetryClientErrors is the behaviour fix: an expired presigned URL
// returns 403 and must fail immediately rather than burning every attempt.
func TestDoDoesNotRetryClientErrors(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 100)
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()
	uploader.failFor = func(string, int) error {
		return neterr.NewStatusError("upload part", presignedURL, 403,
			[]byte("<Error><Code>AccessDenied</Code><Message>Request has expired</Message></Error>"))
	}

	err := Do(t.Context(), bytes.NewReader(payload), int64(len(payload)), sess, uploader, fastOpts())

	require.Error(t, err)
	assert.Equal(t, 1, uploader.attemptsFor(1),
		"a 403 must not be retried")
	assert.Contains(t, err.Error(), "upload link expired")
	assert.NotContains(t, err.Error(), "X-Amz-Signature")
}

func TestDoRetriesServerErrorsThenSucceeds(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 100)
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()
	uploader.failFor = func(_ string, attempt int) error {
		if attempt < 3 {
			return neterr.NewStatusError("upload part", presignedURL, 503, nil)
		}
		return nil
	}

	require.NoError(t, Do(t.Context(), bytes.NewReader(payload), int64(len(payload)), sess, uploader, fastOpts()))
	assert.Equal(t, 3, uploader.attemptsFor(1))
}

func TestDoRetriesTransportErrors(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 100)
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()
	uploader.failFor = func(_ string, attempt int) error {
		if attempt == 1 {
			return &url.Error{Op: "Put", URL: presignedURL, Err: syscall.ECONNRESET}
		}
		return nil
	}

	require.NoError(t, Do(t.Context(), bytes.NewReader(payload), int64(len(payload)), sess, uploader, fastOpts()))
	assert.Equal(t, 2, uploader.attemptsFor(1))
}

func TestDoPartFailureIdentifiesThePart(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 500)
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()
	uploader.failFor = func(gotURL string, _ int) error {
		if gotURL == partURL(4) {
			return neterr.NewStatusError("upload part", presignedURL, 403, nil)
		}
		return nil
	}

	err := Do(t.Context(), bytes.NewReader(payload), int64(len(payload)), sess, uploader, fastOpts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "part 4 of 5")
	assert.Contains(t, err.Error(), "upload build artifact")
	assert.NotContains(t, err.Error(), "X-Amz-Signature")

	// The 403 is terminal, so the failing part is tried exactly once. Sibling
	// parts are deliberately not asserted on: once the group's context is
	// cancelled, whether a given part started at all is a scheduling race.
	assert.Equal(t, 1, uploader.attemptsFor(4))
}

func TestDoDoesNotCompleteWhenAPartFails(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 300)
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()
	uploader.failFor = func(string, int) error {
		return neterr.NewStatusError("upload part", presignedURL, 403, nil)
	}

	require.Error(t, Do(t.Context(), bytes.NewReader(payload), int64(len(payload)), sess, uploader, fastOpts()))
	assert.Empty(t, sess.completed, "a partial upload must never be completed")
}

func TestDoInitiateFailureIsScrubbed(t *testing.T) {
	sess := &fakeSession{
		initiateErr: fmt.Errorf("boom talking to %s", presignedURL),
	}

	err := Do(t.Context(), bytes.NewReader([]byte("x")), 1, sess, newFakeUploader(), fastOpts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not start upload")
	assert.NotContains(t, err.Error(), "X-Amz-Signature")
}

func TestDoCompleteFailureIsScrubbed(t *testing.T) {
	sess := &fakeSession{
		uploadID:    "u",
		completeErr: fmt.Errorf("assemble failed for %s", presignedURL),
	}

	err := Do(t.Context(), bytes.NewReader([]byte("x")), 1, sess, newFakeUploader(), fastOpts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "could not finalise upload")
	assert.NotContains(t, err.Error(), "X-Amz-Signature")
}

func TestDoRejectsEmptyPartList(t *testing.T) {
	err := Do(t.Context(), bytes.NewReader(nil), 0, &emptyPartsSession{}, newFakeUploader(), fastOpts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "no upload targets")
}

type emptyPartsSession struct{}

func (e *emptyPartsSession) Initiate(context.Context, int) (*api.InitiateUploadResponse, error) {
	return &api.InitiateUploadResponse{UploadID: "u"}, nil
}
func (e *emptyPartsSession) Complete(context.Context, string, []api.PartInfo) error { return nil }

func TestDoCancellationStopsUpload(t *testing.T) {
	payload := bytes.Repeat([]byte("x"), 1000)
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()
	uploader.delay = 20 * time.Millisecond

	ctx, cancel := context.WithCancel(t.Context())
	cancel()

	err := Do(ctx, bytes.NewReader(payload), int64(len(payload)), sess, uploader, fastOpts())

	require.Error(t, err)
	assert.True(t, errors.Is(err, context.Canceled), "got %v", err)
	assert.Empty(t, sess.completed)
}

// failingReaderAt simulates a local disk read error.
type failingReaderAt struct{}

func (failingReaderAt) ReadAt([]byte, int64) (int, error) {
	return 0, errors.New("disk read failure")
}

func TestDoLocalReadErrorIsNotRetried(t *testing.T) {
	sess := &fakeSession{uploadID: "u"}
	uploader := newFakeUploader()

	err := Do(t.Context(), failingReaderAt{}, 100, sess, uploader, fastOpts())

	require.Error(t, err)
	assert.Contains(t, err.Error(), "read part 1")
	assert.Zero(t, uploader.attemptsFor(1))
}
