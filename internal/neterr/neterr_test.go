package neterr

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"os"
	"strings"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// presignedURL mirrors the shape of a real upload URL: the query string carries
// live credentials that must never reach a log or a terminal.
const presignedURL = "https://cerebrium-app-storage-us-prod.s3.us-east-1.amazonaws.com/p-test-123/cortex/test-app/build-abc123.zip" +
	"?X-Amz-Algorithm=AWS4-HMAC-SHA256" +
	"&X-Amz-Credential=ASIATESTCREDENTIAL%2F20260803%2Fus-east-1%2Fs3%2Faws4_request" +
	"&X-Amz-Date=20260803T182244Z" +
	"&X-Amz-Expires=600" +
	"&X-Amz-Security-Token=TESTSECURITYTOKENVALUE" +
	"&X-Amz-SignedHeaders=host" +
	"&x-id=PutObject" +
	"&X-Amz-Signature=7486f33dddcbec78d1f4c5fa30ef0f3d0ff2d0103574b0b7f9101ded176b816e"

// secrets are the substrings that must never appear in any user-facing output.
var secrets = []string{
	"X-Amz-Signature",
	"X-Amz-Credential",
	"X-Amz-Security-Token",
	"7486f33dddcbec78d1f4c5fa30ef0f3d0ff2d0103574b0b7f9101ded176b816e",
	"TESTSECURITYTOKENVALUE",
	"ASIATESTCREDENTIAL",
}

func assertNoSecrets(t *testing.T, s string) {
	t.Helper()
	for _, secret := range secrets {
		assert.NotContains(t, s, secret, "leaked credential material")
	}
}

func TestRedactURL(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "presigned url keeps object path drops credentials",
			in:   presignedURL,
			want: "https://cerebrium-app-storage-us-prod.s3.us-east-1.amazonaws.com/p-test-123/cortex/test-app/build-abc123.zip?<redacted>",
		},
		{
			name: "url without query is unchanged",
			in:   "https://rest.cerebrium.ai/v2/projects/p-test-123/apps",
			want: "https://rest.cerebrium.ai/v2/projects/p-test-123/apps",
		},
		{
			name: "userinfo is stripped",
			in:   "https://user:pass@example.com/path",
			want: "https://example.com/path",
		},
		{
			name: "unparseable url is cut at the query separator",
			in:   "ht tp://bad host/path?X-Amz-Signature=deadbeef",
			want: "ht tp://bad host/path?<redacted>",
		},
		{
			name: "empty string",
			in:   "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RedactURL(tt.in)
			assert.Equal(t, tt.want, got)
			assertNoSecrets(t, got)
		})
	}
}

// TestScrubRealWorldError uses the verbatim error text a customer reported, to
// prove the safety net catches text we never explicitly built.
func TestScrubRealWorldError(t *testing.T) {
	raw := fmt.Sprintf(`upload failed: Put %q: write tcp 10.1.10.2:49785->16.15.254.12:443: write: broken pipe`, presignedURL)

	got := Scrub(raw)

	assertNoSecrets(t, got)
	// The useful parts survive.
	assert.Contains(t, got, "broken pipe")
	assert.Contains(t, got, "build-abc123.zip")
}

func TestScrubLeavesNonURLTextAlone(t *testing.T) {
	in := "no urls here, just a question mark? and a=b pair"
	assert.Equal(t, in, Scrub(in))
}

func TestScrubTrailingPunctuation(t *testing.T) {
	got := Scrub("see https://example.com/a?X-Amz-Signature=abc.")
	assert.Equal(t, "see https://example.com/a?<redacted>.", got)
}

// wrapTransport builds the error shape net/http actually returns: a *url.Error
// carrying the full URL, wrapping a *net.OpError, wrapping a syscall error.
func wrapTransport(inner error) error {
	return &url.Error{
		Op:  "Put",
		URL: presignedURL,
		Err: &net.OpError{
			Op:     "write",
			Net:    "tcp",
			Source: &net.TCPAddr{IP: net.ParseIP("10.1.10.2"), Port: 49785},
			Addr:   &net.TCPAddr{IP: net.ParseIP("16.15.254.12"), Port: 443},
			Err:    os.NewSyscallError("write", inner),
		},
	}
}

func TestWrapClassifiesTransportFailures(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		wantSummary string
		wantHint    string
	}{
		{
			name:        "connection reset",
			err:         wrapTransport(syscall.ECONNRESET),
			wantSummary: "the connection was reset to cerebrium-app-storage-us-prod.s3.us-east-1.amazonaws.com",
			wantHint:    "local network, VPN or proxy",
		},
		{
			name:        "broken pipe",
			err:         wrapTransport(syscall.EPIPE),
			wantSummary: "the connection closed while data was still being sent",
			wantHint:    "local network, VPN or proxy",
		},
		{
			name:        "connection refused",
			err:         wrapTransport(syscall.ECONNREFUSED),
			wantSummary: "the connection was refused",
			wantHint:    "proxy or firewall",
		},
		{
			name:        "network unreachable",
			err:         wrapTransport(syscall.ENETUNREACH),
			wantSummary: "the network is unreachable",
			wantHint:    "network connection or VPN",
		},
		{
			name:        "unexpected eof",
			err:         &url.Error{Op: "Put", URL: presignedURL, Err: io.ErrUnexpectedEOF},
			wantSummary: "the connection ended before the transfer finished",
			wantHint:    "local network, VPN or proxy",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := Wrap("upload build artifact", presignedURL, tt.err)
			require.Error(t, err)

			msg := err.Error()
			assertNoSecrets(t, msg)
			assert.Contains(t, msg, "upload build artifact")
			assert.Contains(t, msg, tt.wantSummary)
			if tt.wantHint != "" {
				assert.Contains(t, msg, tt.wantHint)
			}
			// The object path stays visible so the user knows what failed.
			assert.Contains(t, msg, "build-abc123.zip")
		})
	}
}

func TestWrapPreservesErrorIdentity(t *testing.T) {
	original := wrapTransport(syscall.ECONNRESET)

	err := Wrap("upload build artifact", presignedURL, original)

	// Callers can still match on the underlying cause.
	assert.True(t, errors.Is(err, syscall.ECONNRESET))
	var opErr *net.OpError
	assert.True(t, errors.As(err, &opErr))
}

func TestWrapTimeout(t *testing.T) {
	err := Wrap("upload build artifact", presignedURL, &url.Error{
		Op:  "Put",
		URL: presignedURL,
		Err: &net.OpError{Op: "dial", Net: "tcp", Err: &timeoutError{}},
	})

	msg := err.Error()
	assertNoSecrets(t, msg)
	assert.Contains(t, msg, "timed out")
}

type timeoutError struct{}

func (t *timeoutError) Error() string   { return "i/o timeout" }
func (t *timeoutError) Timeout() bool   { return true }
func (t *timeoutError) Temporary() bool { return true }

func TestWrapDNSFailure(t *testing.T) {
	err := Wrap("upload build artifact", presignedURL, &url.Error{
		Op:  "Put",
		URL: presignedURL,
		Err: &net.DNSError{Err: "no such host", Name: "cerebrium-app-storage-us-prod.s3.us-east-1.amazonaws.com"},
	})

	msg := err.Error()
	assertNoSecrets(t, msg)
	assert.Contains(t, msg, "DNS lookup")
	assert.Contains(t, msg, "cerebrium-app-storage-us-prod.s3.us-east-1.amazonaws.com")
}

func TestWrapTLSFailureSuggestsProxy(t *testing.T) {
	err := Wrap("upload build artifact", presignedURL, &url.Error{
		Op:  "Put",
		URL: presignedURL,
		Err: x509.UnknownAuthorityError{},
	})

	msg := err.Error()
	assertNoSecrets(t, msg)
	assert.Contains(t, msg, "unknown authority")
	assert.Contains(t, msg, "corporate proxy")
}

func TestWrapCancelled(t *testing.T) {
	err := Wrap("upload build artifact", presignedURL, context.Canceled)
	assert.Contains(t, err.Error(), "cancelled")
}

func TestWrapNilIsNil(t *testing.T) {
	assert.NoError(t, Wrap("op", presignedURL, nil))
}

func TestWrapDoesNotDoubleWrap(t *testing.T) {
	status := NewStatusError("upload", presignedURL, 403, []byte("AccessDenied"))
	assert.Same(t, error(status), Wrap("upload", presignedURL, status))

	inner := Wrap("upload", presignedURL, wrapTransport(syscall.EPIPE))
	assert.Same(t, inner, Wrap("upload", presignedURL, inner))
}

// TestWrapUnknownCauseStillScrubs guards the fallback branch: even a cause we
// do not recognise must not carry a signature through.
func TestWrapUnknownCauseStillScrubs(t *testing.T) {
	err := Wrap("upload build artifact", presignedURL, &url.Error{
		Op:  "Put",
		URL: presignedURL,
		Err: fmt.Errorf("something odd happened talking to %s", presignedURL),
	})

	assertNoSecrets(t, err.Error())
}

func TestStatusError(t *testing.T) {
	tests := []struct {
		name          string
		status        int
		body          string
		wantContains  string
		wantHint      string
		wantRetryable bool
	}{
		{
			name:          "expired signature is explained and not retried",
			status:        403,
			body:          "<Error><Code>AccessDenied</Code><Message>Request has expired</Message></Error>",
			wantContains:  "upload link expired",
			wantHint:      "re-run the command",
			wantRetryable: false,
		},
		{
			name:          "plain forbidden",
			status:        403,
			body:          "<Error><Code>SignatureDoesNotMatch</Code></Error>",
			wantContains:  "not authorised",
			wantRetryable: false,
		},
		{
			name:          "too large",
			status:        413,
			wantContains:  "too large",
			wantRetryable: false,
		},
		{
			name:          "service unavailable is retryable",
			status:        503,
			wantContains:  "temporarily unavailable",
			wantRetryable: true,
		},
		{
			name:          "internal error is retryable",
			status:        500,
			wantContains:  "temporarily unavailable",
			wantRetryable: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := NewStatusError("upload build artifact", presignedURL, tt.status, []byte(tt.body))

			msg := err.Error()
			assertNoSecrets(t, msg)
			assert.Contains(t, msg, tt.wantContains)
			assert.Contains(t, msg, fmt.Sprintf("HTTP %d", tt.status))
			if tt.wantHint != "" {
				assert.Contains(t, msg, tt.wantHint)
			}
			assert.Equal(t, tt.wantRetryable, err.Retryable())
			assert.Equal(t, tt.wantRetryable, Retryable(err))
		})
	}
}

func TestStatusErrorScrubsBody(t *testing.T) {
	// A service that echoes the request URL back must not defeat redaction.
	err := NewStatusError("upload", presignedURL, 400, []byte("bad request for "+presignedURL))
	assertNoSecrets(t, err.Error())
	assertNoSecrets(t, err.Body)
}

func TestStatusErrorTruncatesLongBodies(t *testing.T) {
	err := NewStatusError("upload", presignedURL, 500, []byte(strings.Repeat("x", 4096)))
	assert.Len(t, err.Body, 512+len("…"))
	assert.True(t, strings.HasSuffix(err.Body, "…"))
}

func TestRetryable(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want bool
	}{
		{name: "nil", err: nil, want: false},
		{name: "cancelled", err: context.Canceled, want: false},
		{name: "deadline exceeded", err: context.DeadlineExceeded, want: false},
		{name: "wrapped cancellation", err: fmt.Errorf("part 3: %w", context.Canceled), want: false},
		{name: "client error", err: NewStatusError("op", presignedURL, 403, nil), want: false},
		{name: "server error", err: NewStatusError("op", presignedURL, 503, nil), want: true},
		{name: "transport error", err: wrapTransport(syscall.ECONNRESET), want: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Retryable(tt.err))
		})
	}
}

func TestSanitizeError(t *testing.T) {
	original := fmt.Errorf("upload failed: %w", wrapTransport(syscall.EPIPE))

	err := SanitizeError(original)

	require.Error(t, err)
	assertNoSecrets(t, err.Error())
	// Identity is preserved for callers matching on the cause.
	assert.True(t, errors.Is(err, syscall.EPIPE))
}

func TestSanitizeErrorPassesThroughCleanErrors(t *testing.T) {
	original := errors.New("nothing sensitive here")
	assert.Same(t, original, SanitizeError(original))
}

func TestSanitizeErrorNil(t *testing.T) {
	assert.NoError(t, SanitizeError(nil))
}
