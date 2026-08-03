// Package upload drives S3 multipart uploads against presigned part URLs.
//
// The transport is identical wherever we upload — chunk the file, PUT each part
// to its presigned URL concurrently, retry the transient failures, then tell the
// backend to assemble them. Only the initiate/complete calls differ between an
// upload to a project volume and an upload of a build artifact, so those are
// supplied by a Session and everything else is shared.
package upload

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync/atomic"
	"time"

	"github.com/avast/retry-go/v4"
	"golang.org/x/sync/errgroup"

	"github.com/cerebriumai/cerebrium/internal/api"
	"github.com/cerebriumai/cerebrium/internal/neterr"
)

// Defaults used when Options leaves a field zero.
const (
	DefaultPartSize    = 5 * 1024 * 1024
	DefaultConcurrency = 10
	DefaultMaxAttempts = 3
	DefaultRetryDelay  = 2 * time.Second
)

// Session performs the backend calls that bracket a multipart upload. Each
// upload destination (project volume, build artifact, …) provides its own.
type Session interface {
	// Initiate reserves a multipart upload and returns an upload ID plus one
	// presigned URL per part, in part-number order.
	Initiate(ctx context.Context, partCount int) (*api.InitiateUploadResponse, error)
	// Complete assembles the uploaded parts into the final object.
	Complete(ctx context.Context, uploadID string, parts []api.PartInfo) error
}

// PartUploader PUTs one part's bytes to a presigned URL and returns its ETag.
type PartUploader interface {
	UploadPart(ctx context.Context, url string, data []byte) (string, error)
}

// Options tunes a multipart upload. Zero fields take the package defaults.
type Options struct {
	// PartSize is the byte size of every part except the last.
	PartSize int64
	// Concurrency caps how many parts are in flight at once. Peak memory is
	// roughly PartSize × Concurrency, since each in-flight part is buffered so
	// it can be replayed on retry.
	Concurrency int
	// MaxAttempts is the total number of tries per part, including the first.
	MaxAttempts int
	// RetryDelay is the base delay for exponential backoff between attempts.
	RetryDelay time.Duration
	// Progress, when non-nil, accumulates bytes as parts complete.
	Progress *atomic.Int64
	// Op names the operation in error messages, e.g. "upload build artifact".
	Op string
}

func (o Options) withDefaults() Options {
	if o.PartSize <= 0 {
		o.PartSize = DefaultPartSize
	}
	if o.Concurrency <= 0 {
		o.Concurrency = DefaultConcurrency
	}
	if o.MaxAttempts <= 0 {
		o.MaxAttempts = DefaultMaxAttempts
	}
	if o.RetryDelay <= 0 {
		o.RetryDelay = DefaultRetryDelay
	}
	if o.Op == "" {
		o.Op = "upload"
	}
	return o
}

// PartCount returns the number of parts a file of the given size needs.
//
// An empty file still reports one part: the backend expects at least one part
// URL, and sending zero would select a different server-side code path.
func PartCount(size, partSize int64) int {
	if partSize <= 0 {
		partSize = DefaultPartSize
	}
	count := int((size + partSize - 1) / partSize)
	if count == 0 {
		count = 1
	}
	return count
}

// Do uploads size bytes read from r as a multipart upload.
//
// r must support concurrent ReadAt calls; *os.File does, which is why parts are
// read positionally rather than through a shared seek offset.
func Do(ctx context.Context, r io.ReaderAt, size int64, sess Session, uploader PartUploader, opts Options) error {
	opts = opts.withDefaults()

	partCount := PartCount(size, opts.PartSize)

	initResp, err := sess.Initiate(ctx, partCount)
	if err != nil {
		return fmt.Errorf("%s: could not start upload: %w", opts.Op, neterr.SanitizeError(err))
	}
	if initResp == nil || len(initResp.Parts) == 0 {
		return fmt.Errorf("%s: backend returned no upload targets", opts.Op)
	}

	results, err := uploadParts(ctx, r, size, initResp.Parts, uploader, opts)
	if err != nil {
		return err
	}

	if err := sess.Complete(ctx, initResp.UploadID, results); err != nil {
		return fmt.Errorf("%s: could not finalise upload: %w", opts.Op, neterr.SanitizeError(err))
	}

	return nil
}

// uploadParts uploads every part concurrently and returns the ETags in
// part-number order.
func uploadParts(
	ctx context.Context,
	r io.ReaderAt,
	size int64,
	parts []api.PartURL,
	uploader PartUploader,
	opts Options,
) ([]api.PartInfo, error) {
	results := make([]api.PartInfo, len(parts))

	eg, ctx := errgroup.WithContext(ctx)
	eg.SetLimit(opts.Concurrency)

	for i, part := range parts {
		idx, part := i, part

		eg.Go(func() error {
			etag, n, err := uploadOnePart(ctx, r, size, part, uploader, opts)
			if err != nil {
				return fmt.Errorf("%s: part %d of %d failed: %w", opts.Op, part.PartNumber, len(parts), err)
			}

			results[idx] = api.PartInfo{PartNumber: part.PartNumber, ETag: etag}
			if opts.Progress != nil {
				opts.Progress.Add(n)
			}
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	return results, nil
}

// uploadOnePart reads a part's bytes and PUTs them, retrying transient
// failures. It returns the part's ETag and the number of bytes uploaded.
func uploadOnePart(
	ctx context.Context,
	r io.ReaderAt,
	size int64,
	part api.PartURL,
	uploader PartUploader,
	opts Options,
) (string, int64, error) {
	data, err := readPart(r, size, part.PartNumber, opts.PartSize)
	if err != nil {
		// A local read failure will not fix itself; do not retry it.
		return "", 0, err
	}

	var etag string
	err = retry.Do(
		func() error {
			etag, err = uploader.UploadPart(ctx, part.URL, data)
			return err
		},
		retry.Context(ctx),
		retry.Attempts(uint(opts.MaxAttempts)),
		retry.Delay(opts.RetryDelay),
		retry.DelayType(retry.BackOffDelay),
		retry.LastErrorOnly(true),
		// Only retry what a retry could plausibly fix. A 4xx — an expired
		// signature above all — fails identically every time, so retrying it
		// only delays the message the user needs.
		retry.RetryIf(neterr.Retryable),
	)
	if err != nil {
		return "", 0, err
	}

	return etag, int64(len(data)), nil
}

// readPart reads the byte range belonging to partNumber (1-based).
func readPart(r io.ReaderAt, size int64, partNumber int, partSize int64) ([]byte, error) {
	offset := int64(partNumber-1) * partSize

	length := partSize
	if remaining := size - offset; remaining < length {
		length = remaining
	}
	if length < 0 {
		length = 0
	}

	buf := make([]byte, length)
	if length == 0 {
		return buf, nil
	}

	n, err := r.ReadAt(buf, offset)
	// ReadAt may report EOF alongside a full read when the range ends exactly at
	// end-of-input; that is success, not failure.
	if err != nil && (!errors.Is(err, io.EOF) || int64(n) != length) {
		return nil, fmt.Errorf("read part %d: %w", partNumber, err)
	}

	return buf[:n], nil
}
