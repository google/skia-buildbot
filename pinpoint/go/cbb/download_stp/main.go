// A command-line tool to download Safari Technology Preview (STP) for CBB use.
//
// Already implemented:
// * Parse STP resources page to find latest STP release number and download links.
// * Download STP installation images, create CIPD packages from them,
//   and add ref labels needed to trigger installtion on CBB devices.
//
// TODO(b/433796487):
// * Update CBB ref file.
// * Trigger CBB runs.

package main

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"go.skia.org/infra/go/httputils"
	"golang.org/x/net/html"
)

const macosVersion = "macos15"
const cipdPath = "infra/chromeperf/cbb/safari_technology_preview"

var httpClient = httputils.NewTimeoutClient()

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

// Check if an existing CIPD package with the right reference already exists.
func findCipd(ref string) bool {
	cmd := exec.Command("cipd", "resolve", cipdPath, "-version", ref)
	err := cmd.Run()

	if err == nil {
		// No error means "cipd resolve" found an existing package.
		return true
	}

	exitError, isExitError := err.(*exec.ExitError)
	if isExitError && exitError.ProcessState.ExitCode() == 1 {
		// Exit code 1 means "cipd resolve" did not find an existing package.
		return false
	}

	log.Fatalf("Unexpected error from cipd resolve: %v", err)
	return false
}

// Download an STP installation image, and create a CIPD package form it.
func createCipd(url string, refs []string) error {
	// Create a new temporary directory
	tmpDir, err := os.MkdirTemp("", "stp")
	if err != nil {
		return fmt.Errorf("error creating temp dir: %w\n", err)
	}
	// Ignore errors during clean-up.
	defer func() { _ = os.RemoveAll(tmpDir) }()

	// Download the file
	resp, err := httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("error downloading from %s: %w\n", url, err)
	}
	defer resp.Body.Close()

	// Create the file inside the temporary directory
	dmgPath := filepath.Join(tmpDir, "SafariTechnologyPreview.dmg")
	tmpfile, err := os.Create(dmgPath)
	if err != nil {
		return fmt.Errorf("error creating temp file %s: %w\n", dmgPath, err)
	}

	// Write the body to the file
	_, err = io.Copy(tmpfile, resp.Body)
	if err != nil {
		return fmt.Errorf("error writing to temp file %s: %w\n", dmgPath, err)
	}
	tmpfile.Close()

	// Upload to CIPD
	args := []string{
		"create", "-in", tmpDir, "-name", cipdPath,
	}
	for _, ref := range refs {
		args = append(args, "-ref", ref)
	}
	cmd := exec.Command("cipd", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("error running cipd %v: %w\n", args, err)
	}

	fmt.Printf("Successfully uploaded %s to CIPD at %s with refs %v\n", dmgPath, cipdPath, refs)
	return nil
}

func main() {
	doc, err := downloadAndParseHtml()
	if err != nil {
		fmt.Fprintf(os.Stderr, "unable to download and parse HTML: %v", err)
		os.Exit(1)
	}

	ri := extractFromHtml(doc)
	if ri.release == "" || ri.linkSequoia == "" || ri.linkTahoe == "" {
		if ri.release == "" {
			fmt.Fprintln(os.Stderr, "Unable to discover STP release number")
		}
		if ri.linkSequoia == "" {
			fmt.Fprintln(os.Stderr, "Unable to discover STP download link for Sequoia")
		}
		if ri.linkTahoe == "" {
			fmt.Fprintln(os.Stderr, "Unable to discover STP download link for Tahoe")
		}
		os.Exit(1)
	}
	fmt.Printf("Release: %s\n", ri.release)
	fmt.Printf("Download Links:\n  %s\n  %s\n", ri.linkTahoe, ri.linkSequoia)

	if found := findCipd(ri.release + "-" + macosVersion); found {
		fmt.Printf("Current STP release %s has already been downloaded, exiting...\n", ri.release)
		return
	}

	// The "stable" ref causes the new package to be installed on CBB lab devices.
	// The "canary" ref is needed due to the way lab is set up, and it should generally
	// point to the same version as the "stable" ref.
	// The other refs are for informational purposes only.
	err = createCipd(ri.linkSequoia, []string{ri.release + "-macos15", "stable", "canary", "latest"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create CIPD package for macOS 15: %v", err)
		os.Exit(1)
	}
	err = createCipd(ri.linkTahoe, []string{ri.release + "-macos26"})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to create CIPD package for macOS 26: %v", err)
		os.Exit(1)
	}
}
