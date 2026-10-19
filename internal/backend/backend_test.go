package backend

import (
	"context"
	"testing"

	"github.com/juanrubio/quotary/internal/domain"
)

type fakeBackend struct{}

func (fakeBackend) Version(context.Context) (VersionInfo, error) {
	return VersionInfo{Version: "test"}, nil
}

func (fakeBackend) Catalog(context.Context) (CatalogSnapshot, error) {
	return CatalogSnapshot{Providers: []domain.SourceDescriptor{{ProviderID: "codex"}}}, nil
}

func (fakeBackend) Usage(context.Context, UsageRequest) ([]domain.UsageSnapshot, error) {
	return []domain.UsageSnapshot{{ProviderID: "codex"}}, nil
}

func (fakeBackend) History(context.Context, HistoryRequest) (domain.HistorySnapshot, error) {
	zero := int64(0)
	return domain.HistorySnapshot{TotalTokens: &zero}, nil
}

func TestInterfaceContract(t *testing.T) {
	var backend Backend = fakeBackend{}
	if _, err := backend.Version(context.Background()); err != nil {
		t.Fatal(err)
	}
	catalog, err := backend.Catalog(context.Background())
	if err != nil || len(catalog.Providers) != 1 {
		t.Fatalf("catalog = %+v, %v", catalog, err)
	}
}
