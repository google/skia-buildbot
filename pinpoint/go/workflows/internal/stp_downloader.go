// Downloader for Safari Technology Preview (STP).

package internal

import (
	"context"
	"io"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"go.chromium.org/luci/cipd/client/cipd/pkg"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
	"golang.org/x/net/html"

	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/backends"
)

const (
	// Where to store the CIPD package in production:
	// https://chrome-infra-packages.appspot.com/p/infra/chromeperf/cbb/safari_technology_preview
	cipdPathProd = "infra/chromeperf/cbb/safari_technology_preview"

	// Where to store the CIPD package during testing / experimenting:
	// https://chrome-infra-packages.appspot.com/p/experimental/chromeperf/cbb/safari_technology_preview
	cipdPathExp = "experimental/chromeperf/cbb/safari_technology_preview"

	// Name of the STP installation package file.
	dmgFilename = "SafariTechnologyPreview.dmg"
)

var (
	// Apple labels each download link "Safari Technology Preview for macOS 27".
	// html.Parse turns &nbsp; into U+00A0, which \s doesn't match.
	macOSInLinkLabel = regexp.MustCompile(`macOS[\s\x{00A0}]+(\d+)`)

	// Swarming reports a Mac's OS as a list such as
	// ["Mac", "Mac-27", "Mac-27.0", "Mac-27.0.1", "Mac-27.0.1-26A434"].
	// The pattern has to match the whole value, because the build number
	// starts with the Darwin version (26 on macOS 27).
	macOSMajorDimension = regexp.MustCompile(`^Mac-(\d+)$`)
)

// Download the Safari Technology Preview resources page, and then parse its HTML contents.
func downloadAndParseHtml(ctx context.Context) (*html.Node, error) {
	resp, err := httputils.GetWithContext(ctx, httpClient, "https://developer.apple.com/safari/resources/")
	if err != nil {
		return nil, skerr.Wrapf(err, "error downloading STP resource page")
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, skerr.Wrapf(err, "error parsing HTML")
	}

	return doc, nil
}

type releaseInfo struct {
	// links maps a macOS major version, e.g. "27", to the STP download link
	// that Apple publishes for it.
	links map[string]string

	// The STP release number, e.g., "255" as of October 22, 2026.
	release string
}

// Extract releaseInfo from downloaded STP resource page.
func extractFromHtml(doc *html.Node) *releaseInfo {
	ri := releaseInfo{links: map[string]string{}}
	var findInfo func(*html.Node)
	findInfo = func(n *html.Node) {
		// Find Release Number, using the pattern <div class="column"> <p>Release</p> <p>123</p> </div>.
		if n.Type == html.ElementNode && n.Data == "div" {
			isColumnDiv := false
			for _, attr := range n.Attr {
				if attr.Key == "class" && strings.Contains(attr.Val, "column") {
					isColumnDiv = true
					break
				}
			}

			if isColumnDiv {
				var childElements []*html.Node
				for c := n.FirstChild; c != nil; c = c.NextSibling {
					if c.Type == html.ElementNode {
						childElements = append(childElements, c)
					}
				}

				if len(childElements) == 2 {
					firstChild := childElements[0]
					secondChild := childElements[1]

					var firstChildText string
					if firstChild.FirstChild != nil && firstChild.FirstChild.Type == html.TextNode {
						firstChildText = firstChild.FirstChild.Data
					}

					if strings.Contains(firstChildText, "Release") {
						if secondChild.FirstChild != nil && secondChild.FirstChild.Type == html.TextNode {
							ri.release = strings.TrimSpace(secondChild.FirstChild.Data)
						}
					}
				}
			}
		}

		// Find Download Links
		if n.Type == html.ElementNode && n.Data == "a" {
			for _, a := range n.Attr {
				if a.Key == "href" && strings.HasSuffix(a.Val, ".dmg") &&
					(strings.Contains(a.Val, "SafariTechnologyPreview") || strings.Contains(a.Val, "SafariTechPreview")) &&
					n.LastChild != nil {
					if m := macOSInLinkLabel.FindStringSubmatch(n.LastChild.Data); m != nil {
						ri.links[m[1]] = a.Val
					}
				}
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			findInfo(c)
		}
	}
	findInfo(doc)

	return &ri
}

// fleetMacOSMajor returns the macOS major version of the live CBB Mac bots.
// An STP build installs only on the macOS version it was built for, and the
// "stable" CIPD ref can point to one build, so a fleet in the middle of an
// upgrade is an error rather than a guess.
func fleetMacOSMajor(ctx context.Context, sc backends.SwarmingClient) (string, error) {
	majors := map[string]bool{}
	for config := range safariConfigs {
		bots, err := sc.FetchFreeBots(ctx, config)
		if err != nil {
			return "", skerr.Wrapf(err, "unable to list bots for %s", config)
		}
		for _, bot := range bots {
			if major := botMacOSMajor(bot); major != "" {
				majors[major] = true
			}
		}
	}
	found := slices.Sorted(maps.Keys(majors))
	if len(found) != 1 {
		return "", skerr.Fmt("want one macOS major version on the live CBB Mac bots, got %v", found)
	}
	return found[0], nil
}

// botMacOSMajor returns the macOS major version in a bot's "os" dimension, or
// "" if the bot doesn't report one.
func botMacOSMajor(bot *apipb.BotInfo) string {
	for _, dim := range bot.GetDimensions() {
		if dim.GetKey() != "os" {
			continue
		}
		for _, v := range dim.GetValue() {
			if m := macOSMajorDimension.FindStringSubmatch(v); m != nil {
				return m[1]
			}
		}
	}
	return ""
}

// prepareCipd makes all the preparations needed to access the CIPD repository.
// It returns a CIPD Client object, path to the directory where temp files are
// stored (caller should clean up this directory after using CPID), and
// possible error.
func prepareCipd(ctx context.Context) (*cipd.Client, string, error) {
	// Create a new temporary directory
	tmpDir, err := os.MkdirTemp("", "cipd")
	if err != nil {
		return nil, "", skerr.Wrapf(err, "error creating temp dir for CIPD")
	}

	cipdClient, err := cipd.NewClient(ctx, tmpDir, cipd.DefaultServiceURL)
	if err != nil {
		return nil, "", skerr.Wrapf(err, "unable to create CIPD client")
	}

	return cipdClient, tmpDir, nil
}

// createCipd downloads an STP installation image, and creates a CIPD package form it.
func createCipd(ctx context.Context, cipdClient *cipd.Client, cipdPath, url string, refs []string) error {
	// Create a new temporary directory
	tmpDir, err := os.MkdirTemp("", "stp")
	if err != nil {
		return skerr.Wrapf(err, "error creating temp dir")
	}
	// Ignore errors during clean-up.
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Download the file
	resp, err := httputils.GetWithContext(ctx, httpClient, url)
	if err != nil {
		return skerr.Wrapf(err, "error downloading from %s", url)
	}
	defer resp.Body.Close()

	// Create the file inside the temporary directory
	dmgPath := filepath.Join(tmpDir, dmgFilename)
	tmpfile, err := os.Create(dmgPath)
	if err != nil {
		return skerr.Wrapf(err, "error creating temp file %s", dmgPath)
	}

	// Write the body to the file
	_, err = io.Copy(tmpfile, resp.Body)
	if err != nil {
		return skerr.Wrapf(err, "error writing to temp file %s", dmgPath)
	}
	tmpfile.Close()

	// Upload to CIPD
	_, err = cipdClient.Create(ctx, cipdPath, tmpDir, pkg.InstallModeSymlink, nil, refs, nil, nil)
	if err != nil {
		return skerr.Wrapf(err, "error creating CIPD package")
	}

	sklog.Infof("Successfully uploaded %s to CIPD at %s with refs %v\n", dmgPath, cipdPath, refs)
	return nil
}

// DownloadSafariTPActivity is a Temporal activity that does the following:
//   - Parse STP resources page to find latest STP release number and download links.
//   - Read the macOS major version of the CBB Mac bots from Swarming, and pick
//     the download link for that version.
//   - Download STP installation images, create CIPD packages from them,
//     and add ref labels needed to trigger installtion on CBB devices.
//   - Returns the current STP release number.
func DownloadSafariTPActivity(ctx context.Context, isDev bool) (string, error) {
	doc, err := downloadAndParseHtml(ctx)
	if err != nil {
		return "", skerr.Wrapf(err, "unable to download and parse STP site HTML")
	}

	ri := extractFromHtml(doc)
	if ri.release == "" {
		return "", skerr.Fmt("unable to discover STP release number")
	}

	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		return "", skerr.Wrapf(err, "unable to create Swarming client")
	}
	macos, err := fleetMacOSMajor(ctx, sc)
	if err != nil {
		return "", err
	}
	link, ok := ri.links[macos]
	if !ok {
		return "", skerr.Fmt("STP release %s has no download for macOS %s, which the CBB Mac bots run; Apple lists macOS %v",
			ri.release, macos, slices.Sorted(maps.Keys(ri.links)))
	}
	versionRef := ri.release + "-macos" + macos

	sklog.Infof("Release: %s\n", ri.release)
	sklog.Infof("Download link for macOS %s:\n  %s\n", macos, link)

	cipdClient, cipdRootPath, err := prepareCipd(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(cipdRootPath) }()

	cipdPath := cipdPathProd
	if isDev {
		cipdPath = cipdPathExp
	}
	_, err = cipdClient.ResolveVersion(ctx, cipdPath, versionRef)
	if err == nil {
		sklog.Infof("Current STP release %s has already been downloaded, skipping.", versionRef)
		return ri.release, nil
	}
	if err.Error() != "no such ref" {
		return "", skerr.Wrapf(err, "unexpected error while looking up existing CIPD package")
	}
	sklog.Infof("Downloading STP release %s", versionRef)

	// The "stable" ref causes the new package to be installed on CBB lab devices.
	// The "canary" ref is needed due to the way lab is set up, and it should generally
	// point to the same version as the "stable" ref.
	// The other refs are for informational purposes only.
	err = createCipd(
		ctx, cipdClient, cipdPath, link,
		[]string{versionRef, "stable", "canary", "latest"},
	)
	if err != nil {
		return "", skerr.Wrapf(err, "unable to create CIPD package %s", versionRef)
	}

	return ri.release, nil
}

var downloadSafariTPActivityOptions = workflow.ActivityOptions{
	StartToCloseTimeout: 5 * time.Minute,
	RetryPolicy: &temporal.RetryPolicy{
		InitialInterval:    15 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    2 * time.Minute,
		MaximumAttempts:    1,
	},
}

// DownloadSafariTPWorkflow is a Temporal workflow that calls DownloadSafariTPActivity.
// This workflow is intended for dev testing only, not for production, thus we always pass
// isDev=true to the underlying activity.
func DownloadSafariTPWorkflow(ctx workflow.Context) (string, error) {
	ctx = workflow.WithActivityOptions(ctx, downloadSafariTPActivityOptions)
	var version string
	err := workflow.ExecuteActivity(ctx, DownloadSafariTPActivity, true).Get(ctx, &version)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return version, nil
}
