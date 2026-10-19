package tui

import (
	"github.com/juanrubio/quotary/internal/backend"
	"github.com/juanrubio/quotary/internal/domain"
)

func sanitizeSnapshot(snapshot domain.UsageSnapshot) domain.UsageSnapshot {
	snapshot.DisplayName = cleanText(snapshot.DisplayName)
	snapshot.Plan = cleanText(snapshot.Plan)
	snapshot.AccountLabel = cleanText(snapshot.AccountLabel)
	snapshot.ReportedSource = cleanText(snapshot.ReportedSource)
	for i := range snapshot.Windows {
		snapshot.Windows[i].Label = cleanText(snapshot.Windows[i].Label)
		snapshot.Windows[i].Group = cleanText(snapshot.Windows[i].Group)
		snapshot.Windows[i].Unit = cleanText(snapshot.Windows[i].Unit)
	}
	for i := range snapshot.Balances {
		snapshot.Balances[i].Label = cleanText(snapshot.Balances[i].Label)
		snapshot.Balances[i].Unit = cleanText(snapshot.Balances[i].Unit)
	}
	for i := range snapshot.Details {
		snapshot.Details[i].Title = cleanText(snapshot.Details[i].Title)
		for j := range snapshot.Details[i].Rows {
			snapshot.Details[i].Rows[j].Label = cleanText(snapshot.Details[i].Rows[j].Label)
			snapshot.Details[i].Rows[j].Value = cleanText(snapshot.Details[i].Rows[j].Value)
		}
		for j := range snapshot.Details[i].Bars {
			snapshot.Details[i].Bars[j].Label = cleanText(snapshot.Details[i].Bars[j].Label)
			snapshot.Details[i].Bars[j].Unit = cleanText(snapshot.Details[i].Bars[j].Unit)
		}
		for j := range snapshot.Details[i].Points {
			snapshot.Details[i].Points[j].Label = cleanText(snapshot.Details[i].Points[j].Label)
		}
	}
	snapshot.State = domain.EffectiveState(snapshot)
	return snapshot
}

func sanitizeCatalog(catalog backend.CatalogSnapshot) backend.CatalogSnapshot {
	for i := range catalog.Providers {
		catalog.Providers[i].DisplayName = cleanText(catalog.Providers[i].DisplayName)
	}
	return catalog
}

func sanitizeHistory(history domain.HistorySnapshot) domain.HistorySnapshot {
	history.Provenance = cleanText(history.Provenance)
	history.Coverage = cleanText(history.Coverage)
	history.Timezone = cleanText(history.Timezone)
	if history.Cost != nil {
		history.Cost.Currency = cleanText(history.Cost.Currency)
	}
	for i := range history.Models {
		history.Models[i].Name = cleanText(history.Models[i].Name)
	}
	for i := range history.Buckets {
		history.Buckets[i].Day = cleanText(history.Buckets[i].Day)
	}
	for i := range history.MissingDays {
		history.MissingDays[i] = cleanText(history.MissingDays[i])
	}
	return history
}

func safeError(err error) string {
	if scoped, ok := domain.AsScopedError(err); ok {
		return scoped.Error()
	}
	return "This action could not be completed."
}
