// Downloader for Safari Technology Preview (STP).

package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"go.chromium.org/luci/cipd/client/cipd/pkg"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"golang.org/x/net/html"
)

const (
	cipdPathProd = "infra/chromeperf/cbb/safari_technology_preview"
	cipdPathExp  = "experimental/chromeperf/cbb/safari_technology_preview"
	macosVersion = "macos15"
	dmgFilename  = "SafariTechnologyPreview.dmg"
)

// Download the Safari Technology Preview resources page, and then parse its HTML contents.
func downloadAndParseHtml() (*html.Node, error) {
	resp, err := httpClient.Get("https://developer.apple.com/safari/resources/")
	if err != nil {
		return nil, fmt.Errorf("error downloading STP resource page: %w\n", err)
	}
	defer resp.Body.Close()

	doc, err := html.Parse(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing HTML: %w\n", err)
	}

	return doc, nil
}

type releaseInfo struct {
	release     string
	linkTahoe   string
	linkSequoia string
}

// Extract releaseInfo from downloaded STP resource page.
func extractFromHtml(doc *html.Node) *releaseInfo {
	var ri releaseInfo
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
				if a.Key == "href" && strings.Contains(a.Val, "SafariTechnologyPreview.dmg") {
					osInfo := n.LastChild.Data
					if strings.Contains(osInfo, "Tahoe") {
						ri.linkTahoe = a.Val
					} else if strings.Contains(osInfo, "Sequoia") {
						ri.linkSequoia = a.Val
					} else {
						fmt.Fprintf(os.Stderr, "Unable to discover macOS version: %s\n", n.LastChild.Data)
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
func createCipd(ctx context.Context, cipdClient *cipd.Client, cipdPath string, url string, refs []string) error {
	// Create a new temporary directory
	tmpDir, err := os.MkdirTemp("", "stp")
	if err != nil {
		return skerr.Wrapf(err, "error creating temp dir")
	}
	// Ignore errors during clean-up.
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Download the file
	resp, err := httpClient.Get(url)
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
//   - Download STP installation images, create CIPD packages from them,
//     and add ref labels needed to trigger installtion on CBB devices.
//   - Returns the current STP release number.
func DownloadSafariTPActivity(ctx context.Context, isDev bool) (string, error) {
	doc, err := downloadAndParseHtml()
	if err != nil {
		return "", skerr.Wrapf(err, "unable to download and parse STP site HTML")
	}

	ri := extractFromHtml(doc)
	if ri.release == "" {
		return "", fmt.Errorf("unable to discover STP release number")
	}
	if ri.linkSequoia == "" {
		return "", fmt.Errorf("unable to discover STP download link for Sequoia")
	}
	if ri.linkTahoe == "" {
		return "", fmt.Errorf("unable to discover STP download link for Tahoe")
	}

	sklog.Infof("Release: %s\n", ri.release)
	sklog.Infof("Download Links:\n  %s\n  %s\n", ri.linkTahoe, ri.linkSequoia)

	cipdClient, cipdRootPath, err := prepareCipd(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = os.RemoveAll(cipdRootPath) }()

	cipdPath := cipdPathProd
	if isDev {
		cipdPath = cipdPathExp
	}
	_, err = cipdClient.ResolveVersion(ctx, cipdPath, ri.release+"-"+macosVersion)
	if err == nil {
		sklog.Infof("Current STP release %s has already been downloaded, skipping.", ri.release)
		return ri.release, nil
	}
	if err.Error() != "no such ref" {
		return "", skerr.Wrapf(err, "unexpected error while looking up existing CIPD package")
	}
	sklog.Infof("Downloading STP release %s", ri.release)

	// The "stable" ref causes the new package to be installed on CBB lab devices.
	// The "canary" ref is needed due to the way lab is set up, and it should generally
	// point to the same version as the "stable" ref.
	// The other refs are for informational purposes only.
	err = createCipd(
		ctx, cipdClient, cipdPath, ri.linkSequoia,
		[]string{ri.release + "-macos15", "stable", "canary", "latest"})
	if err != nil {
		return "", skerr.Wrapf(err, "unable to create CIPD package for MacOS 15")
	}
	err = createCipd(
		ctx, cipdClient, cipdPath, ri.linkTahoe, []string{ri.release + "-macos26"})
	if err != nil {
		return "", skerr.Wrapf(err, "unable to create CIPD package for MacOS 26")
	}

	return ri.release, nil
}
