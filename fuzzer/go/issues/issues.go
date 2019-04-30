package issues

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/go/issues"
)

// bugReportingPackage is a struct containing the pieces of a fuzz that may need to have
// a bug filed or updated.
type IssueReportingPackage struct {
	FuzzName       string
	CommitRevision string
	Category       string
}

type IssuesManager struct {
	client *http.Client
}

func NewManager(c *http.Client) *IssuesManager {
	return &IssuesManager{
		client: c,
	}
}

type newBug struct {
	Category       string
	PrettyCategory string
	Description    string
	Name           string
	Hash           string
	Revision       string
	Params         string
}

var newBugTemplate = template.Must(template.New("new_bug").Parse(`# Description here about fuzz found in {{.PrettyCategory}}
{{.Description}}

To replicate, build target "fuzz" at the specified commit and run:
out/Release/fuzz {{.Params}} ~/Downloads/{{.Name}}

The problem may only be revealed by an ASAN build, in which case you would need to run:
gn gen out/ASAN --args='cc="/usr/bin/clang" cxx="/usr/bin/clang++" sanitize="ASAN"'
or:
gn gen out/ASAN --args='cc="/usr/bin/clang" cxx="/usr/bin/clang++" sanitize="ASAN" is_debug=false'

prior to building.

# tracking metadata below:
fuzz_category: {{.Category}}
fuzz_commit: {{.Revision}}
related_fuzz: https://fuzzer.skia.org/category/{{.Category}}/name/{{.Hash}}
fuzz_download: https://fuzzer.skia.org/fuzz/{{.Hash}}
`))

func (im *IssuesManager) CreateBadBugIssue(p IssueReportingPackage, desc string) error {
	tracker := issues.NewMonorailIssueTracker(im.client, issues.PROJECT_SKIA)

	m, err := issueMessage(p, desc)
	if err != nil {
		return err
	}

	req := issues.IssueRequest{
		Labels:      append(common.ExtraBugLabels(p.Category), "FromSkiaFuzzer", "Restrict-View-Google", "Type-Defect", "Priority-Medium"),
		Status:      "New",
		Summary:     "New crash found in " + common.PrettifyCategory(p.Category) + " by fuzzer",
		Description: m,
		CC: []issues.MonorailPerson{
			{
				Name: "kjlubick@google.com",
			},
		},
	}
	if groomer := common.Groomer(p.Category); groomer != common.UNCLAIMED {
		req.Owner = issues.MonorailPerson{
			Name: groomer + "@google.com",
		}
	}

	return tracker.AddIssue(req)
}

func (im *IssuesManager) CreateBadBugURL(p IssueReportingPackage) (string, error) {
	// Monorail expects a single, comma seperated list of query params for labels.
	labels := append(common.ExtraBugLabels(p.Category), "FromSkiaFuzzer", "Restrict-View-Google", "Type-Defect", "Priority-Medium")
	q := url.Values{
		"labels":  []string{strings.Join(labels, ",")},
		"status":  []string{"New"},
		"summary": []string{"New crash found in " + common.PrettifyCategory(p.Category) + " by fuzzer"},
		"cc":      []string{"kjlubick@google.com"},
	}

	if groomer := common.Groomer(p.Category); groomer != common.UNCLAIMED {
		q["owner"] = []string{groomer + "@google.com"}
	}

	m, err := issueMessage(p, "")
	if err != nil {
		return "", err
	}
	q.Add("comment", m)

	return "https://bugs.chromium.org/p/skia/issues/entry?" + q.Encode(), nil
}

func issueMessage(p IssueReportingPackage, desc string) (string, error) {
	b := newBug{
		Category:       p.Category,
		PrettyCategory: common.PrettifyCategory(p.Category),
		Description:    desc,
		Name:           common.CategoryReminder(p.Category) + "-" + p.FuzzName,
		Hash:           p.FuzzName,
		Params:         common.ReplicationArgs(p.Category),
		Revision:       p.CommitRevision,
	}
	var t bytes.Buffer
	if err := newBugTemplate.Execute(&t, b); err != nil {
		return "", fmt.Errorf("Could not create template with %#v", b)
	}
	return t.String(), nil
}
