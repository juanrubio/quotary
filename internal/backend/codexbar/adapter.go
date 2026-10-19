package codexbar

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/domain"
)

const (
	usageCap   = 4 << 20
	historyCap = 32 << 20
	versionCap = 1 << 20
	catalogCap = 4 << 20

	versionTimeout = 5 * time.Second
	catalogTimeout = 10 * time.Second
	usageTimeout   = 60 * time.Second
	probeTimeout   = 120 * time.Second
	historyTimeout = 90 * time.Second
)

// Options configures executable resolution and injectable seams.
type Options struct {
	Path           string
	EnvPath        string
	Runner         Runner
	LookPath       func(string) (string, error)
	NativeOnly     bool
	Now            func() time.Time
	VariantAllowed func(provider string, variant domain.Variant) bool
}

// Adapter implements backend.Backend through the CodexBar CLI.
type Adapter struct {
	path           string
	runner         Runner
	nativeOnly     bool
	now            func() time.Time
	variantAllowed func(string, domain.Variant) bool
}

// New resolves the executable once. Explicit invalid paths are setup errors.
func New(opts Options) (*Adapter, error) {
	runner := opts.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	now := opts.Now
	if now == nil {
		now = func() time.Time { return time.Now().UTC() }
	}
	variantAllowed := opts.VariantAllowed
	if variantAllowed == nil {
		variantAllowed = defaultVariantAllowed
	}
	lookPath := opts.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	envPath := opts.EnvPath
	if envPath == "" {
		envPath = os.Getenv("QUOTARY_CODEXBAR_PATH")
	}
	candidates := []struct {
		path     string
		explicit bool
	}{
		{envPath, true},
		{opts.Path, true},
	}
	for _, candidate := range candidates {
		if strings.TrimSpace(candidate.path) == "" {
			continue
		}
		resolved, err := filepath.Abs(candidate.path)
		if err != nil || !isExecutable(resolved) {
			return nil, scoped(domain.ErrConfig, "", "configured CodexBar path is not executable")
		}
		return &Adapter{path: resolved, runner: runner, nativeOnly: opts.NativeOnly, now: now, variantAllowed: variantAllowed}, nil
	}
	resolved, err := lookPath("codexbar")
	if err != nil || !isExecutable(resolved) {
		return nil, scoped(domain.ErrConfig, "", "codexbar executable was not found")
	}
	absolute, err := filepath.Abs(resolved)
	if err != nil {
		return nil, scoped(domain.ErrConfig, "", "codexbar path could not be resolved")
	}
	return &Adapter{path: absolute, runner: runner, nativeOnly: opts.NativeOnly, now: now, variantAllowed: variantAllowed}, nil
}

func defaultVariantAllowed(provider string, variant domain.Variant) bool {
	if variant == "" || variant == domain.VariantConfigured {
		return true
	}
	if provider == "gemini" {
		return variant == domain.VariantWeb || variant == domain.VariantAPI
	}
	return false
}

func (a *Adapter) allowedVariant() func(string, domain.Variant) bool {
	if a.variantAllowed != nil {
		return a.variantAllowed
	}
	return defaultVariantAllowed
}

func scopeDecodeError(err error, key string) error {
	if err == nil {
		return nil
	}
	if scopedErr, ok := domain.AsScopedError(err); ok {
		return domain.NewScopedError(scopedErr.Category, key, scopedErr.Detail)
	}
	return err
}

func (a *Adapter) clock() time.Time {
	if a.now != nil {
		return a.now()
	}
	return time.Now().UTC()
}

func (a *Adapter) validateRequest(provider string, variant domain.Variant, sourceKey string) (domain.SourceKey, error) {
	if !validProviderID(provider) {
		return domain.SourceKey{}, scoped(domain.ErrUnsupported, sourceKey, "invalid provider identifier")
	}
	key := domain.SourceKey{Provider: provider, Variant: variant}
	if sourceKey != "" {
		parsed, err := domain.ParseSourceKey(sourceKey)
		if err != nil {
			return domain.SourceKey{}, scoped(domain.ErrUnsupported, sourceKey, "invalid source key")
		}
		if provider != "" && parsed.Provider != provider {
			return domain.SourceKey{}, scoped(domain.ErrUnsupported, sourceKey, "source key provider does not match request")
		}
		if variant != "" && parsed.Variant != variant {
			return domain.SourceKey{}, scoped(domain.ErrUnsupported, sourceKey, "source key variant does not match request")
		}
		key = parsed
	}
	allowed := a.variantAllowed
	if allowed == nil {
		allowed = defaultVariantAllowed
	}
	if !allowed(key.Provider, key.Variant) {
		return domain.SourceKey{}, scoped(domain.ErrUnsupported, key.String(), "provider does not support this source variant")
	}
	return key, nil
}

func validProviderID(provider string) bool {
	if provider == "" {
		return false
	}
	for index, r := range provider {
		valid := (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || (index > 0 && r == '-')
		if !valid {
			return false
		}
	}
	return true
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

// Path exposes the resolved executable for diagnostics without invoking it.
func (a *Adapter) Path() string { return a.path }

func (a *Adapter) runRaw(ctx context.Context, timeout time.Duration, cap int, args ...string) ([]byte, []byte, error) {
	if err := ctx.Err(); err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, nil, ErrTimeout
		}
		return nil, nil, err
	}
	runCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	stdout, stderr, err := a.runner.Run(runCtx, a.path, args, cap)
	if err == nil && len(stdout) > cap {
		return stdout, stderr, ErrOutputCap
	}
	return stdout, stderr, err
}

func (a *Adapter) run(ctx context.Context, timeout time.Duration, cap int, key string, args ...string) ([]byte, error) {
	stdout, stderr, err := a.runRaw(ctx, timeout, cap, args...)
	if err != nil {
		return stdout, classifyRunError(err, stderr, key)
	}
	return stdout, nil
}

func (a *Adapter) Version(ctx context.Context) (backend.VersionInfo, error) {
	stdout, stderr, rawErr := a.runRaw(ctx, versionTimeout, versionCap, "--version")
	version := strings.TrimSpace(string(stdout))
	if version != "" {
		lines := strings.Split(version, "\n")
		info := backend.VersionInfo{Version: strings.TrimSpace(lines[0])}
		if rawErr != nil {
			var process *ProcessError
			if errors.As(rawErr, &process) {
				return info, classifyRunError(rawErr, stderr, "")
			}
			return backend.VersionInfo{}, classifyRunError(rawErr, stderr, "")
		}
		return info, nil
	}
	if rawErr != nil {
		return backend.VersionInfo{}, classifyRunError(rawErr, stderr, "")
	}
	return backend.VersionInfo{}, scoped(domain.ErrEmptyStdout, "", "version command returned no data")
}

func (a *Adapter) Catalog(ctx context.Context) (backend.CatalogSnapshot, error) {
	stdout, stderr, rawErr := a.runRaw(ctx, catalogTimeout, catalogCap, "config", "providers", "--format", "json")
	catalog, parseErr := parseCatalog(stdout)
	if rawErr != nil {
		var process *ProcessError
		if errors.As(rawErr, &process) && parseErr == nil {
			return catalog, classifyRunError(rawErr, stderr, "")
		}
		return backend.CatalogSnapshot{}, classifyRunError(rawErr, stderr, "")
	}
	return catalog, parseErr
}

func (a *Adapter) Usage(ctx context.Context, req backend.UsageRequest) ([]domain.UsageSnapshot, error) {
	canonical, err := a.validateRequest(req.ProviderID, req.Variant, req.SourceKey)
	if err != nil {
		return nil, err
	}
	req.ProviderID = canonical.Provider
	req.Variant = canonical.Variant
	req.SourceKey = canonical.String()
	key := canonical.String()
	args := []string{"usage", "--provider", canonical.Provider, "--format", "json"}
	if canonical.Variant != "" && canonical.Variant != domain.VariantConfigured {
		args = append(args, "--source", string(canonical.Variant))
	}
	timeout := usageTimeout
	if canonical.Provider == "antigravity" {
		timeout = probeTimeout
	}
	stdout, stderr, rawErr := a.runRaw(ctx, timeout, usageCap, args...)
	snapshots, parseErr := func() ([]domain.UsageSnapshot, error) {
		document, decodeErr := decodeDocument(stdout)
		if decodeErr != nil {
			return nil, scopeDecodeError(decodeErr, key)
		}
		return normalizeUsage(document, req, a.clock(), a.allowedVariant())
	}()
	if parseErr != nil && len(snapshots) > 0 {
		if rawErr != nil {
			var process *ProcessError
			if !errors.As(rawErr, &process) {
				return snapshots, classifyRunError(rawErr, stderr, key)
			}
		}
		return snapshots, parseErr
	}
	if parseErr == nil {
		if rawErr != nil {
			var process *ProcessError
			if errors.As(rawErr, &process) {
				// Valid rows survive a nonzero process exit, together with the
				// failed operation outcome. Fatal timeout/cap failures are
				// returned before parsing can mask them.
				return snapshots, classifyRunError(rawErr, stderr, key)
			}
			return nil, classifyRunError(rawErr, stderr, key)
		}
		return snapshots, nil
	}
	if rawErr != nil {
		var process *ProcessError
		if errors.As(rawErr, &process) {
			if scopedErr, ok := domain.AsScopedError(parseErr); ok && scopedErr.Category != domain.ErrEmptyStdout {
				return nil, scopedErr
			}
		}
		return nil, classifyRunError(rawErr, stderr, key)
	}
	if len(stderr) > 0 {
		if category := categoryFromText(string(stderr)); category != "" {
			return nil, scoped(category, key, "backend reported an error")
		}
	}
	return nil, parseErr
}

func (a *Adapter) History(ctx context.Context, req backend.HistoryRequest) (domain.HistorySnapshot, error) {
	if req.Days != 7 && req.Days != 30 {
		return domain.HistorySnapshot{}, scoped(domain.ErrUnsupported, req.SourceKey, "history range must be 7 or 30 days")
	}
	canonical, err := a.validateRequest(req.ProviderID, req.Variant, req.SourceKey)
	if err != nil {
		return domain.HistorySnapshot{}, err
	}
	req.ProviderID = canonical.Provider
	req.Variant = canonical.Variant
	req.SourceKey = canonical.String()
	key := canonical.String()
	args := []string{"cost", "--provider", canonical.Provider, "--format", "json", "--days", fmt.Sprintf("%d", req.Days)}
	native := canonical.Provider == "codex" || canonical.Provider == "claude"
	if native && a.nativeOnly {
		args = append(args, "--provider-native-only")
	}
	stdout, stderr, rawErr := a.runRaw(ctx, historyTimeout, historyCap, args...)
	document, decodeErr := decodeDocument(stdout)
	var normalizeErr error
	if decodeErr == nil {
		history, err := normalizeHistory(document, req)
		if err == nil {
			if rawErr != nil {
				var process *ProcessError
				if !errors.As(rawErr, &process) {
					return domain.HistorySnapshot{}, classifyRunError(rawErr, stderr, key)
				}
			}
			if native {
				if a.nativeOnly {
					setHistoryScope(&history, "provider-native local records")
				} else {
					setHistoryScope(&history, "all local records (provider-native-only not requested)")
				}
			}
			if rawErr != nil {
				var process *ProcessError
				if errors.As(rawErr, &process) {
					return history, classifyRunError(rawErr, stderr, key)
				}
			}
			return history, nil
		}
		if hasHistorySummary(history) {
			if rawErr != nil {
				var process *ProcessError
				if !errors.As(rawErr, &process) {
					return domain.HistorySnapshot{}, classifyRunError(rawErr, stderr, key)
				}
			}
			if native {
				if a.nativeOnly {
					setHistoryScope(&history, "provider-native local records")
				} else {
					setHistoryScope(&history, "all local records (provider-native-only not requested)")
				}
			}
			return history, err
		}
		normalizeErr = err
	}
	if rawErr != nil {
		// Only an explicit unsupported-option failure may retry without the
		// provider-native-only flag.
		if native && a.nativeOnly && len(args) > 0 && args[len(args)-1] == "--provider-native-only" && unsupportedOption(rawErr, stderr) {
			fallback := append([]string(nil), args[:len(args)-1]...)
			stdout, fallbackStderr, fallbackRawErr := a.runRaw(ctx, historyTimeout, historyCap, fallback...)
			fallbackDocument, fallbackDecodeErr := decodeDocument(stdout)
			if fallbackDecodeErr == nil {
				history, fallbackNormalizeErr := normalizeHistory(fallbackDocument, req)
				if fallbackNormalizeErr == nil {
					setHistoryScope(&history, "all local records (provider-native-only unavailable)")
					if fallbackRawErr != nil {
						var process *ProcessError
						if !errors.As(fallbackRawErr, &process) {
							return domain.HistorySnapshot{}, classifyRunError(fallbackRawErr, fallbackStderr, key)
						}
						return history, classifyRunError(fallbackRawErr, fallbackStderr, key)
					}
					return history, nil
				}
				if hasHistorySummary(history) {
					if fallbackRawErr != nil {
						var process *ProcessError
						if !errors.As(fallbackRawErr, &process) {
							return domain.HistorySnapshot{}, classifyRunError(fallbackRawErr, fallbackStderr, key)
						}
					}
					setHistoryScope(&history, "all local records (provider-native-only unavailable)")
					if fallbackRawErr != nil {
						return history, classifyRunError(fallbackRawErr, fallbackStderr, key)
					}
					return history, fallbackNormalizeErr
				}
				if fallbackRawErr != nil {
					var process *ProcessError
					if !errors.As(fallbackRawErr, &process) {
						return domain.HistorySnapshot{}, classifyRunError(fallbackRawErr, fallbackStderr, key)
					}
				}
				return domain.HistorySnapshot{}, fallbackNormalizeErr
			}
			if fallbackRawErr != nil {
				var process *ProcessError
				if !errors.As(fallbackRawErr, &process) {
					return domain.HistorySnapshot{}, classifyRunError(fallbackRawErr, fallbackStderr, key)
				}
			}
			if fallbackRawErr != nil {
				return domain.HistorySnapshot{}, classifyRunError(fallbackRawErr, fallbackStderr, key)
			}
			if fallbackDecodeErr != nil {
				return domain.HistorySnapshot{}, scopeDecodeError(fallbackDecodeErr, key)
			}
		}
		var process *ProcessError
		if errors.As(rawErr, &process) && normalizeErr != nil {
			return domain.HistorySnapshot{}, normalizeErr
		}
		return domain.HistorySnapshot{}, classifyRunError(rawErr, stderr, key)
	}
	if normalizeErr != nil {
		return domain.HistorySnapshot{}, normalizeErr
	}
	return domain.HistorySnapshot{}, scopeDecodeError(decodeErr, key)
}

func setHistoryScope(history *domain.HistorySnapshot, scope string) {
	if history.Provenance == "" {
		history.Provenance = scope
		return
	}
	if !strings.Contains(history.Provenance, scope) {
		history.Provenance += "; " + scope
	}
}

func hasHistorySummary(history domain.HistorySnapshot) bool {
	return len(history.Buckets) > 0 || history.TotalTokens != nil || history.Cost != nil ||
		len(history.Models) > 0 || history.Coverage != ""
}

func unsupportedOption(err error, stderr []byte) bool {
	var process *ProcessError
	if !errors.As(err, &process) {
		return false
	}
	text := lowerASCII(string(stderr) + " " + string(process.Stderr))
	return containsAny(text, "unknown option", "unknown flag", "unrecognized option", "unrecognized flag", "unsupported option", "unsupported flag")
}

var _ backend.Backend = (*Adapter)(nil)
