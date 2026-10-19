// Package backend defines the contract implemented by live and demo backends.
package backend

import (
	"context"

	"github.com/juanrubio/quotary/internal/domain"
)

// VersionInfo identifies the backend executable.
type VersionInfo struct {
	Version string
}

// CatalogSnapshot is the discovered provider catalog.
type CatalogSnapshot struct {
	Providers []domain.SourceDescriptor
}

// UsageRequest asks for one source and optional account.
type UsageRequest struct {
	ProviderID string
	Variant    domain.Variant
	SourceKey  string
}

// HistoryRequest asks for local history for one source.
type HistoryRequest struct {
	ProviderID string
	Variant    domain.Variant
	SourceKey  string
	Days       int
}

// Backend is implemented by the CodexBar adapter and the demo backend.
type Backend interface {
	Version(ctx context.Context) (VersionInfo, error)
	Catalog(ctx context.Context) (CatalogSnapshot, error)
	Usage(ctx context.Context, req UsageRequest) ([]domain.UsageSnapshot, error)
	History(ctx context.Context, req HistoryRequest) (domain.HistorySnapshot, error)
}
