// Package neterr turns raw Go transport errors into messages a user can act on,
// and keeps presigned-URL credentials out of them.
//
// Uploads go to presigned S3 URLs whose query string carries live AWS
// credentials (X-Amz-Credential, X-Amz-Security-Token, X-Amz-Signature). Go's
// net/http renders a failure as `Put "<full URL>": <cause>`, so the default
// error text both buries the cause and leaks a usable credential into
// terminals, CI logs and crash reports. Everything here exists to avoid that.
package neterr

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/url"
	"regexp"
	"strings"
	"syscall"
)

// Error is a transport failure described in terms of what the user should do.
//
// Error() deliberately never renders the wrapped error: that is where the
// unredacted URL lives. The cause survives for errors.Is/errors.As via Unwrap.
type Error struct {
	// Op is what was being attempted, e.g. "upload build artifact".
	Op string
	// Endpoint is the redacted destination, or "" when not applicable.
	Endpoint string
	// Summary is a one-line plain-language cause.
	Summary string
	// Hint is optional remediation advice.
	Hint string

	err error
}

func (e *Error) Error() string {
	var b strings.Builder
	if e.Op != "" {
		b.WriteString(e.Op)
		b.WriteString(": ")
	}
	b.WriteString(e.Summary)
	if e.Endpoint != "" {
		b.WriteString("\n  endpoint: ")
		b.WriteString(e.Endpoint)
	}
	if e.Hint != "" {
		b.WriteString("\n  hint: ")
		b.WriteString(e.Hint)
	}
	return b.String()
}

func (e *Error) Unwrap() error { return e.err }

// StatusError is a non-2xx HTTP response. Callers use Status to decide whether
// retrying is worthwhile; the response body is scrubbed before being kept.
type StatusError struct {
	Op       string
	Endpoint string
	Status   int
	Body     string
}

func (e *StatusError) Error() string {
	summary, hint := describeStatus(e.Status, e.Body)

	var b strings.Builder
	if e.Op != "" {
		b.WriteString(e.Op)
		b.WriteString(": ")
	}
	fmt.Fprintf(&b, "%s (HTTP %d)", summary, e.Status)
	if e.Endpoint != "" {
		b.WriteString("\n  endpoint: ")
		b.WriteString(e.Endpoint)
	}
	if hint != "" {
		b.WriteString("\n  hint: ")
		b.WriteString(hint)
	}
	return b.String()
}

// Retryable reports whether retrying the same request could plausibly succeed.
// 4xx responses are terminal: an expired signature or a rejected request will
// fail identically on every attempt, so burning the retry budget only delays
// the error the user needs to see.
func (e *StatusError) Retryable() bool {
	return e.Status < 400 || e.Status >= 500
}

// NewStatusError records a non-2xx response, scrubbing the body of any URLs.
func NewStatusError(op, rawURL string, status int, body []byte) *StatusError {
	return &StatusError{
		Op:       op,
		Endpoint: RedactURL(rawURL),
		Status:   status,
		Body:     truncate(Scrub(string(body)), 512),
	}
}

// Retryable reports whether err is worth retrying. Context cancellation and
// 4xx responses are not; anything else is treated as a transient transport
// fault.
func Retryable(err error) bool {
	if err == nil {
		return false
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	var se *StatusError
	if errors.As(err, &se) {
		return se.Retryable()
	}
	return true
}

// Wrap describes err in user-facing terms, attributing it to rawURL's host
// without exposing rawURL's query string. It returns nil when err is nil.
func Wrap(op, rawURL string, err error) error {
	if err == nil {
		return nil
	}
	// Already classified (e.g. a StatusError from a previous layer) — leave it
	// alone rather than double-wrapping.
	var se *StatusError
	if errors.As(err, &se) {
		return err
	}
	var ne *Error
	if errors.As(err, &ne) {
		return err
	}

	summary, hint := classify(err, hostOf(rawURL))
	return &Error{
		Op:       op,
		Endpoint: RedactURL(rawURL),
		Summary:  summary,
		Hint:     hint,
		err:      err,
	}
}

// classify maps a transport error to a plain-language summary and hint. host
// may be "" when the destination is unknown.
func classify(err error, host string) (summary, hint string) {
	const localNetworkHint = "this is usually a local network, VPN or proxy problem rather than an outage — " +
		"retry, and if it persists test the same network with a large upload elsewhere"

	switch {
	case errors.Is(err, context.Canceled):
		return "cancelled", ""

	case errors.Is(err, syscall.ECONNRESET):
		return connDesc("the connection was reset", host), localNetworkHint

	case errors.Is(err, syscall.EPIPE):
		return connDesc("the connection closed while data was still being sent", host), localNetworkHint

	case errors.Is(err, syscall.ECONNREFUSED):
		return connDesc("the connection was refused", host), "check for a proxy or firewall intercepting HTTPS traffic"

	case errors.Is(err, syscall.EHOSTUNREACH), errors.Is(err, syscall.ENETUNREACH):
		return connDesc("the network is unreachable", host), "check your network connection or VPN"

	case errors.Is(err, syscall.ETIMEDOUT), isTimeout(err):
		return connDesc("the request timed out", host), "the link may be too slow or a firewall may be dropping packets silently"

	case errors.Is(err, io.ErrUnexpectedEOF), errors.Is(err, io.EOF):
		return connDesc("the connection ended before the transfer finished", host), localNetworkHint
	}

	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return fmt.Sprintf("DNS lookup for %s failed", dnsErr.Name), "check your DNS resolver or VPN split-DNS configuration"
	}

	var certErr *tls.CertificateVerificationError
	if errors.As(err, &certErr) {
		return "the TLS certificate could not be verified", tlsHint
	}
	var authorityErr x509.UnknownAuthorityError
	if errors.As(err, &authorityErr) {
		return "the TLS certificate was issued by an unknown authority", tlsHint
	}
	var hostnameErr x509.HostnameError
	if errors.As(err, &hostnameErr) {
		return "the TLS certificate does not match the server name", tlsHint
	}

	// Unrecognised: fall back to the scrubbed cause so we never print a
	// signature, while still saying something concrete.
	return Scrub(innermost(err).Error()), ""
}

const tlsHint = "this usually means a corporate proxy is re-signing HTTPS traffic; " +
	"trust its certificate authority or exempt this host"

func connDesc(what, host string) string {
	if host == "" {
		return what
	}
	return fmt.Sprintf("%s to %s", what, host)
}

func isTimeout(err error) bool {
	var ne net.Error
	return errors.As(err, &ne) && ne.Timeout()
}

// describeStatus explains the HTTP statuses these upload paths actually produce.
func describeStatus(status int, body string) (summary, hint string) {
	if status == 403 && isExpiredSignature(body) {
		return "the upload link expired before the transfer finished",
			"re-run the command; if uploads routinely take this long, a slow link is the underlying cause"
	}

	switch status {
	case 400:
		return "the storage service rejected the request", ""
	case 401, 403:
		return "the upload was not authorised", "re-run the command to obtain a fresh upload link"
	case 404:
		return "the upload destination no longer exists", ""
	case 413:
		return "the artifact is too large for a single upload", ""
	case 429:
		return "the storage service is rate limiting the upload", "retry shortly"
	case 500, 502, 503, 504:
		return "the storage service is temporarily unavailable", "retry shortly"
	}
	if status >= 500 {
		return "the storage service returned an error", "retry shortly"
	}
	return "the upload was rejected", ""
}

func isExpiredSignature(body string) bool {
	b := strings.ToLower(body)
	return strings.Contains(b, "request has expired") || strings.Contains(b, "accessdenied") && strings.Contains(b, "expired")
}

// urlPattern matches absolute HTTP(S) URLs embedded in free-form text.
var urlPattern = regexp.MustCompile(`https?://[^\s"'<>\\]+`)

// Scrub removes query strings from every URL in s. It is the safety net for
// error text produced by code paths that do not build an Error explicitly.
func Scrub(s string) string {
	if s == "" {
		return s
	}
	return urlPattern.ReplaceAllStringFunc(s, func(match string) string {
		// Trailing punctuation is part of the surrounding prose, not the URL.
		trailing := ""
		for len(match) > 0 {
			last := match[len(match)-1]
			if last == '.' || last == ',' || last == ')' || last == ':' || last == ';' {
				trailing = string(last) + trailing
				match = match[:len(match)-1]
				continue
			}
			break
		}
		return RedactURL(match) + trailing
	})
}

// RedactURL reduces a URL to scheme://host/path, dropping the query string and
// any userinfo. A URL that cannot be parsed is cut at the first '?' so a
// malformed presigned URL still cannot leak its signature.
func RedactURL(raw string) string {
	if raw == "" {
		return ""
	}

	u, err := url.Parse(raw)
	if err != nil {
		if i := strings.IndexByte(raw, '?'); i >= 0 {
			return raw[:i] + redactedSuffix
		}
		return raw
	}

	hadQuery := u.RawQuery != "" || u.ForceQuery
	u.RawQuery = ""
	u.ForceQuery = false
	u.Fragment = ""
	u.User = nil

	out := u.String()
	if hadQuery {
		out += redactedSuffix
	}
	return out
}

const redactedSuffix = "?<redacted>"

// SanitizeError returns an error whose message carries no URL query strings.
// Use it at boundaries that hand arbitrary errors to a user or a reporting
// service. The original error is preserved for errors.Is/errors.As.
func SanitizeError(err error) error {
	if err == nil {
		return nil
	}
	msg := err.Error()
	clean := Scrub(msg)
	if clean == msg {
		return err
	}
	return &sanitized{msg: clean, err: err}
}

type sanitized struct {
	msg string
	err error
}

func (s *sanitized) Error() string { return s.msg }
func (s *sanitized) Unwrap() error { return s.err }

// hostOf extracts the host from a URL, tolerating unparseable input.
func hostOf(raw string) string {
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return u.Host
}

// innermost walks to the deepest wrapped error, which is where the concrete
// cause lives once net/http's *url.Error layer is stripped away.
func innermost(err error) error {
	for {
		// *url.Error embeds the full URL in its message; skip straight past it.
		var ue *url.Error
		if errors.As(err, &ue) && ue.Err != nil {
			err = ue.Err
			continue
		}
		next := errors.Unwrap(err)
		if next == nil {
			return err
		}
		err = next
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
