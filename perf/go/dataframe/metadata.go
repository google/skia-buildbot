package dataframe

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
)

// GetMetadataForTraces returns the metadata for the traces present in the data frame for the relevant commits.
func GetMetadataForTraces(ctx context.Context, df *DataFrame, metadataStore tracestore.MetadataStore) ([]types.TraceMetadata, error) {
	// Extract the relevant trace ids from the data frame.
	traceIds := []string{}
	sourceFileIds := []int64{}
	for traceId := range df.TraceSet {
		traceIds = append(traceIds, traceId)
	}
	if len(traceIds) == 0 {
		return []types.TraceMetadata{}, nil
	}

	// Collect all the source file ids for which we will extract metadata.
	foundIds := map[int64]bool{}
	for _, traceId := range traceIds {
		sourceMap := df.SourceInfo[traceId]
		if sourceMap != nil {
			for _, sourceFileId := range sourceMap.GetAllSourceFileIds() {
				foundIds[sourceFileId] = true
			}
		}
	}
	for sourceFileId := range foundIds {
		sourceFileIds = append(sourceFileIds, sourceFileId)
	}

	if len(sourceFileIds) == 0 {
		return []types.TraceMetadata{}, nil
	}

	slices.Sort(sourceFileIds)
	// The return value is a map where the key is the source file name and value is the map of links.
	linkInfo, err := metadataStore.GetMetadataForSourceFileIDs(ctx, sourceFileIds)
	if err != nil {
		return nil, skerr.Wrapf(err, "Error getting metadata")
	}

	traceMetadatas := []types.TraceMetadata{}
	// rawLinks is a nested map that tracks links from database for a
	// traceId->commitnum->links lookup.
	rawLinks := map[string]map[types.CommitNumber]map[string]string{}
	for traceId := range df.TraceSet {
		traceMetadata := types.TraceMetadata{
			TraceID:     traceId,
			CommitLinks: map[types.CommitNumber]map[string]types.TraceCommitLink{},
		}

		sourceInfo := df.SourceInfo[traceId]
		if sourceInfo != nil {
			// Populate the raw links for this trace for the relevant commit numbers.
			rawLinks[traceId] = map[types.CommitNumber]map[string]string{}
			for _, commitnum := range sourceInfo.GetAllCommitNumbers() {
				sourceFileId, ok := sourceInfo.Get(commitnum)
				if !ok {
					continue
				}
				links, ok := linkInfo[sourceFileId]
				if !ok {
					continue
				}
				rawLinks[traceId][commitnum] = links
			}
		}
		traceMetadatas = append(traceMetadatas, traceMetadata)
	}

	// Now that we have the metadata and the raw links, let's populate the commit links in
	// the metadata objects based on instance configuration.
	return PopulateTraceMetadataLinksBasedOnConfig(traceMetadatas, df.Header[0].Offset, rawLinks), nil
}

// PopulateTraceMetadataLinksBasedOnConfig populates the commit links for the trace metadata
// objects based on the specifications in the instance configs.
//
// Links which are commits are modified to show commit ranges from prev data points for well known keys.
// Links which are specified as useful in the instance config are added from the raw links.
// All other links are ignored.
func PopulateTraceMetadataLinksBasedOnConfig(traceCommitLinks []types.TraceMetadata, startCommitNumber types.CommitNumber, rawLinks map[string]map[types.CommitNumber]map[string]string) []types.TraceMetadata {
	ret := []types.TraceMetadata{}
	for _, traceCommitLink := range traceCommitLinks {
		for currentCommit, currentCommitLinks := range rawLinks[traceCommitLink.TraceID] {
			traceCommitLink.CommitLinks[currentCommit] = map[string]types.TraceCommitLink{}

			if config.Config == nil {
				for key, href := range currentCommitLinks {
					traceCommitLink.CommitLinks[currentCommit][key] = types.TraceCommitLink{
						Href: href,
					}
				}
				continue
			}

			// Let's find the prev commit for this trace that has data.
			prevCommit := currentCommit - 1
			for prevCommit >= startCommitNumber {
				if _, ok := rawLinks[traceCommitLink.TraceID][prevCommit]; !ok {
					// if no data present, we move on.
					prevCommit = prevCommit - 1
				} else {
					break
				}
			}
			if prevCommit >= startCommitNumber {
				// Define a func that will easily extract the commit id from a commit url.
				getCommitIdFromCommitUrl := func(commitUrl string) string {
					commitIdStartIdx := strings.LastIndex(commitUrl, "/")
					return commitUrl[commitIdStartIdx+1:]
				}
				// Define a func that will easily extract the repo url from the commit url.
				getRepoUrlFromCommitUrl := func(commitUrl string) string {
					repoEndIdx := strings.Index(commitUrl, "+")
					return commitUrl[:repoEndIdx]
				}
				prevCommitLinks := rawLinks[traceCommitLink.TraceID][prevCommit]

				// Let's assemble the commit range urls if specified in the config.
				if len(config.Config.DataPointConfig.KeysForCommitRange) > 0 {
					for _, key := range config.Config.DataPointConfig.KeysForCommitRange {
						currentCommitidInLinks := getCommitIdFromCommitUrl(currentCommitLinks[key])
						if currentCommitidInLinks != "" {
							prevCommitidInLinks := getCommitIdFromCommitUrl(prevCommitLinks[key])
							switch key {
							case "V8":
								currentCommitLinks[key] = "https://chromium.googlesource.com/v8/v8/+/" + currentCommitidInLinks
								if prevCommitLinks != nil {
									prevCommitLinks[key] = "https://chromium.googlesource.com/v8/v8/+/" + prevCommitidInLinks
								}
							case "WebRTC":
								currentCommitLinks[key] = "https://chromium.googlesource.com/external/webrtc/+/" + currentCommitidInLinks
								if prevCommitLinks != nil {
									prevCommitLinks[key] = "https://chromium.googlesource.com/external/webrtc/+/" + prevCommitidInLinks
								}
							}
							var linkValue types.TraceCommitLink
							// There is no relevant change in the commits specified in the links.
							if currentCommitidInLinks == prevCommitidInLinks || prevCommitidInLinks == "" {
								linkValue = types.TraceCommitLink{
									Text: fmt.Sprintf("%s (No Change)", currentCommitidInLinks[:8]),
									Href: currentCommitLinks[key],
								}
							} else {
								// Since the prev commit and current commit are different, show is as a commit range url.
								commitRangeUrl := fmt.Sprintf("%s+log/%s..%s", getRepoUrlFromCommitUrl(currentCommitLinks[key]), prevCommitidInLinks, currentCommitidInLinks)
								linkValue = types.TraceCommitLink{
									Text: fmt.Sprintf("%s - %s", prevCommitidInLinks[:8], currentCommitidInLinks[:8]),
									Href: commitRangeUrl,
								}
							}
							traceCommitLink.CommitLinks[currentCommit][key] = linkValue
						}
					}
				}
			}

			// Let's assemble the other useful links if specified in the config.
			if len(config.Config.DataPointConfig.KeysForUsefulLinks) > 0 {
				for key, link := range currentCommitLinks {
					if slices.Contains(config.Config.DataPointConfig.KeysForUsefulLinks, key) {
						traceCommitLink.CommitLinks[currentCommit][key] = types.TraceCommitLink{
							Href: link,
						}
					}
				}
			}
		}
		ret = append(ret, traceCommitLink)
	}

	return ret
}
