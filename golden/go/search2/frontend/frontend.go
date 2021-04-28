package frontend

import (
	"sort"

	"go.skia.org/infra/golden/go/search2"
)

// ChangelistSummaryResponseV1 is a summary of the results associated with a given CL. It focuses on
// the untriaged and new images produced.
type ChangelistSummaryResponseV1 struct {
	// ChangelistID is the nonqualified id of the CL.
	ChangelistID string `json:"changelist_id"`
	// PatchsetSummaries is a summary for all Patchsets for which we have data.
	PatchsetSummaries []PatchsetNewAndUntriagedSummaryV1 `json:"patchsets"`
	// Outdated will be true if this is a stale cached entry. Clients are free to try again later
	// for the latest results.
	Outdated bool `json:"outdated"`
}

// PatchsetNewAndUntriagedSummaryV1 is the summary for a specific PS. It focuses on the untriaged
// and new images produced.
type PatchsetNewAndUntriagedSummaryV1 struct {
	// NewImages is the number of new images (digests) that were produced by this patchset by
	// non-ignored traces and not seen on the primary branch.
	NewImages int `json:"new_images"`
	// NewUntriagedImages is the number of NewImages which are still untriaged. It is less than or
	// equal to NewImages.
	NewUntriagedImages int `json:"new_untriaged_images"`
	// TotalUntriagedImages is the number of images produced by this patchset by non-ignored traces
	// that are untriaged. This includes images that are untriaged and observed on the primary
	// branch (i.e. might not be the fault of this CL/PS). It is greater than or equal to
	// NewUntriagedImages.
	TotalUntriagedImages int `json:"total_untriaged_images"`
	// PatchsetID is the nonqualified id of the patchset. This is usually a git hash.
	PatchsetID string `json:"patchset_id"`
	// PatchsetOrder is represents the chronological order the patchsets are in. It starts at 1.
	PatchsetOrder int `json:"patchset_order"`
}

// ConvertChangelistSummaryResponseV1 converts the search2 version of a Changelist summary into
// the version expected by the frontend.
func ConvertChangelistSummaryResponseV1(summary search2.NewAndUntriagedSummary) ChangelistSummaryResponseV1 {
	xps := make([]PatchsetNewAndUntriagedSummaryV1, 0, len(summary.PatchsetSummaries))
	for _, ps := range summary.PatchsetSummaries {
		xps = append(xps, PatchsetNewAndUntriagedSummaryV1{
			NewImages:            ps.NewImages,
			NewUntriagedImages:   ps.NewUntriagedImages,
			TotalUntriagedImages: ps.TotalUntriagedImages,
			PatchsetID:           ps.PatchsetID,
			PatchsetOrder:        ps.PatchsetOrder,
		})
	}
	// It is convenient for the UI to have these sorted with the latest patchset first.
	sort.Slice(xps, func(i, j int) bool {
		return xps[i].PatchsetOrder > xps[j].PatchsetOrder
	})
	return ChangelistSummaryResponseV1{
		ChangelistID:      summary.ChangelistID,
		PatchsetSummaries: xps,
		Outdated:          summary.Outdated,
	}
}
