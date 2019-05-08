package ctdiffingestion

import (
	"context"
	"encoding/json"
	"fmt"
	"io"

	"go.skia.org/infra/ct_pixel_diff/go/common"
	"go.skia.org/infra/ct_pixel_diff/go/dynamicdiff"
	"go.skia.org/infra/ct_pixel_diff/go/resultstore"
	"go.skia.org/infra/go/ingestion"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
)

// The JSON output from CT looks like this:
// {
//  “run_id” : “{userid-timestamp}”,
//  “chromium_patch” : “{link to chromium patch}”,
//  "skia_patch" : "{link to skia patch}",
//  “screenshots” : [
//    {
//      “type” : “{nopatch/withpatch}”,
//      "rank" : {popularity rank of site},
//      “filename” : “{GS filename}”,
//      "url" : "{URL of web page}"
//    }, ...
//  ]
// }

const (
	// Possible values for a screenshot's type.
	NO_PATCH   = "nopatch"
	WITH_PATCH = "withpatch"

	// Default png extension for images.
	IMG_EXTENSION = ".png"
)

// Screenshot contains the information for a screenshot taken by CT.
type Screenshot struct {
	// Type specifies whether the screenshot was taken without the patch or with
	// the patch.
	Type string `json:"type"`

	// Rank identifies the popularity rank of the website.
	Rank int `json:"rank"`

	// Filename is the name of the screenshot, as stored in GS.
	Filename string `json:"filename"`

	// URL is the URL of the website.
	URL string `json:"url"`
}

// CTResults is the top level structure for decoding CT pixel diff JSON output.
type CTResults struct {
	// RunID specifies the unique ID for the CT run, in the form userid-timestamp.
	RunID string `json:"run_id"`

	// ChromiumPatch is a link to the Chromium patch used to create the pixel diff
	// run.
	ChromiumPatch string `json:"chromium_patch"`

	// SkiaPatch is a link to the Skia patch used to create the pixel diff run.
	SkiaPatch string `json:"skia_patch"`

	// Screenshots lists the screenshots taken and accounted for in the JSON file.
	Screenshots []*Screenshot `json:"screenshots"`

	// name is the name/path of the file where the data came from.
	name string
}

// Parses CT pixel diff JSON output and returns a CTResults object.
func parseCTResultsFromReader(r io.ReadCloser, name string) (*CTResults, error) {
	defer util.Close(r)

	dec := json.NewDecoder(r)
	ctResults := &CTResults{}
	if err := dec.Decode(ctResults); err != nil {
		return nil, fmt.Errorf("Failed to decode JSON (filename: %s): %s", name, err)
	}
	ctResults.name = name
	return ctResults, nil
}

// pixelDiffProcessor implements the ingestion.Processor interface for CT Pixel
// Diff.
type pixelDiffProcessor struct {
	diffStore   diff.DiffStore
	resultStore resultstore.ResultStore
}

// NewPixelDiffProcessor takes in a DiffStore and a ResultStore to return the
// ingestion.Processor for CT Pixel Diff's ingestion process
func NewPixelDiffProcessor(diffStore diff.DiffStore, resultStore resultstore.ResultStore) (ingestion.Processor, error) {
	return &pixelDiffProcessor{
		diffStore:   diffStore,
		resultStore: resultStore,
	}, nil
}

// See the ingestion.Processor interface.
func (p *pixelDiffProcessor) Process(ctx context.Context, resultsFile ingestion.ResultFileLocation) error {
	r, err := resultsFile.Open()
	if err != nil {
		return err
	}

	// Parse the JSON file.
	results, err := parseCTResultsFromReader(r, resultsFile.Name())
	if err != nil {
		return err
	}

	// Process the screenshots.
	for _, screenshot := range results.Screenshots {
		// Trim the image extension from the filename and create the imageID.
		filename := screenshot.Filename[:len(screenshot.Filename)-len(IMG_EXTENSION)]
		imageID := common.GetImageID(results.RunID, screenshot.Type, filename, screenshot.Rank)

		// Get the entry from the ResultStore using the runID and URL.
		rec, err := p.resultStore.Get(results.RunID, screenshot.URL)
		if err != nil {
			return err
		}

		// If no entry exists, create a new one.
		if rec == nil {
			rec = &resultstore.ResultRec{
				RunID: results.RunID,
				URL:   screenshot.URL,
				Rank:  screenshot.Rank,
			}
		}

		// Update the entry with either the nopatch or withpatch imageID.
		if screenshot.Type == NO_PATCH {
			rec.NoPatchImg = imageID
		} else if screenshot.Type == WITH_PATCH {
			rec.WithPatchImg = imageID
		}

		// Calculate diff metrics if the entry contains both nopatch and withpatch
		// images.
		if rec.HasBothImages() {
			diffResult, err := p.diffStore.Get(diff.PRIORITY_NOW, rec.NoPatchImg, []common.ImageID{rec.WithPatchImg})
			if err != nil {
				return err
			}
			if diffResult[rec.WithPatchImg] != nil {
				rec.DiffMetrics = diffResult[rec.WithPatchImg].(*dynamicdiff.DynamicDiffMetrics)
			}
		}

		// Put the updated entry back into the ResultStore.
		err = p.resultStore.Put(results.RunID, screenshot.URL, rec)
		if err != nil {
			return err
		}
	}

	return nil
}
