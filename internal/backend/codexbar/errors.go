package codexbar

import (
	"context"
	"errors"
	"regexp"
	"strings"

	"github.com/juanrubio/quotary/internal/domain"
)

var (
	ErrTimeout   = errors.New("codexbar timed out")
	ErrOutputCap = errors.New("codexbar output exceeded cap")
)

func scoped(category domain.ErrorCategory, key, detail string) error {
	return domain.NewScopedError(category, key, detail)
}

func classifyRunError(err error, stderr []byte, key string) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.Canceled) {
		return context.Canceled
	}
	if errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded) {
		return scoped(domain.ErrTimeout, key, "backend deadline exceeded")
	}
	if errors.Is(err, ErrOutputCap) {
		return scoped(domain.ErrOutputCap, key, "backend output exceeded configured cap")
	}
	var proc *ProcessError
	if errors.As(err, &proc) {
		if category := categoryFromText(string(stderr) + " " + string(proc.Stderr)); category != "" {
			return scoped(category, key, "backend reported an error")
		}
		return scoped(domain.ErrProcessFailure, key, "backend process failed")
	}
	_ = stderr
	return scoped(domain.ErrProcessFailure, key, "backend process failed")
}

func categoryFromText(text string) domain.ErrorCategory {
	lower := lowerASCII(text)
	switch {
	case containsAny(lower, "auth", "login", "sign in", "sign-in", "unauthorized", "forbidden", "credential", "cookie"):
		return domain.ErrAuth
	case containsAny(lower, "no subscription", "not subscribed", "entitlement", "unavailable", "quota unavailable", "no quota"):
		return domain.ErrUnavailable
	case containsAny(lower, "unsupported", "schema", "unrecognized", "unknown shape", "not supported"):
		return domain.ErrUnsupported
	case containsAny(lower, "timeout", "timed out", "deadline"):
		return domain.ErrTimeout
	case containsAny(lower, "malformed", "invalid json", "decode"):
		return domain.ErrMalformedJSON
	}
	return ""
}

var (
	bearerPattern  = regexp.MustCompile(`(?i)bearer\s+[^\s,;]+`)
	secretPattern  = regexp.MustCompile(`(?i)(token|api[_-]?key|secret|password|cookie)([=:\s]+)([^\s,;]+)`)
	jsonSecret     = regexp.MustCompile(`(?i)("(?:access[_-]?token|refresh[_-]?token|id[_-]?token|auth[_-]?token|session[_-]?token|accessToken|refreshToken|idToken|authToken|sessionToken|token|api[_-]?key|apiKey|secret|password|cookie|authorization|session|auth)"\s*:\s*)"(?:\\.|[^"\\])*"`)
	headerPattern  = regexp.MustCompile(`(?i)(cookie|set-cookie|authorization)\s*:[^\n]*`)
	cookiePattern  = regexp.MustCompile(`(?i)\b(cookie|set-cookie)\s*=\s*("[^"]*"|'[^']*'|[^\s]+)`)
	quotedSecret   = regexp.MustCompile(`(?i)\b(access[_-]?token|refresh[_-]?token|id[_-]?token|idToken|auth[_-]?token|session[_-]?token|authorization|session|password|token|api[_-]?key|apiKey|secret|cookie)\s*[:=]\s*(?:"(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*')`)
	emailPattern   = regexp.MustCompile(`(?i)[a-z0-9._%+-]+@[a-z0-9.-]+\.[a-z]{2,}`)
	jwtPattern     = regexp.MustCompile(`eyJ[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+\.[A-Za-z0-9_-]+`)
	urlCredential  = regexp.MustCompile(`(?i)(https?://)[^/@\s]+@`)
	pathPattern    = regexp.MustCompile(`(^|[^A-Za-z0-9])((?:/[^\s,;\"\']+)|(?:[A-Za-z]:\\[^\s,;\"\']+))`)
	ansiPattern    = regexp.MustCompile(`\x1b\[[0-9;?]*[ -/]*[@-~]`)
	controlPattern = regexp.MustCompile(`[\x00-\x08\x0b\x0c\x0e-\x1f\x7f]`)
)

func redactDiagnostic(text string) string {
	text = ansiPattern.ReplaceAllString(text, "")
	text = controlPattern.ReplaceAllString(text, "")
	text = headerPattern.ReplaceAllString(text, "$1: [redacted]")
	text = cookiePattern.ReplaceAllString(text, "$1=[redacted]")
	text = quotedSecret.ReplaceAllString(text, "$1=[redacted]")
	text = urlCredential.ReplaceAllString(text, "${1}[redacted]@")
	text = bearerPattern.ReplaceAllString(text, "Bearer [redacted]")
	text = jsonSecret.ReplaceAllString(text, "${1}\"[redacted]\"")
	text = secretPattern.ReplaceAllString(text, "${1}${2}[redacted]")
	text = emailPattern.ReplaceAllString(text, "[email]")
	text = jwtPattern.ReplaceAllString(text, "[redacted-token]")
	text = pathPattern.ReplaceAllString(text, "${1}[path]")
	if len(text) > 4096 {
		text = text[:4096]
	}
	return strings.TrimSpace(text)
}
