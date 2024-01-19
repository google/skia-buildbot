package tool

import (
	"context"
	_ "embed" // For embed functionality.
	"encoding/json"
	"io/fs"

	"go.skia.org/infra/go/jsonschema"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

// Audience for the tool, i.e. the group of people that use a tool.
type Audience string

// All Audiences
const (
	Any      Audience = "Any"
	Chrome   Audience = "Chrome"
	ChromeOS Audience = "ChromeOS"
	Android  Audience = "Android"
	PEEPSI   Audience = "PEEPSI"
	Skia     Audience = "Skia"
)

// AllAudienceValues is used to export all the constant values to TS.
var AllAudienceValues = []Audience{
	Any,
	Chrome,
	ChromeOS,
	Android,
	PEEPSI,
	Skia,
}

// AcceptingCustomers describes where the tool is in onboarding new users.
type AcceptingCustomers string

const (
	// AcceptingNo customers.
	AcceptingNo AcceptingCustomers = "No"

	// AcceptingSome customers.
	AcceptingSome AcceptingCustomers = "Conditionally"

	// AcceptingAll customers.
	AcceptingAll AcceptingCustomers = "All"
)

// AllAdoptionStages is used to export all the constant values to TS.
var AllAdoptionStages = []AcceptingCustomers{
	AcceptingAll,
	AcceptingNo,
	AcceptingSome,
}

// Phase of development the tool is in.
type Phase string

const (
	// GA is General Availability
	GA Phase = "GA"

	// Deprecated and should not be used.
	Deprecated Phase = "Deprecated"

	// Preview is unstable and not appropriate for GA usage.
	Preview Phase = "Preview"
)

// AllPhases is used to export all the constant values to TS.
var AllPhases = []Phase{
	GA,
	Deprecated,
	Preview,
}

// Domain groups similar tools together.
type Domain string

// These are very general and a bit vauge, which is intenional.
const (
	Build       Domain = "Build"
	Debugging   Domain = "Debugging"
	Development Domain = "Development"
	Logging     Domain = "Logging"
	Other       Domain = "Other"
	Release     Domain = "Release"
	Security    Domain = "Security"
	Source      Domain = "Source"
	Testing     Domain = "Testing"
)

var AllDomains = []Domain{
	Build,
	Debugging,
	Development,
	Logging,
	Other,
	Release,
	Security,
	Source,
	Testing,
}

// Tool describes a single tool.
type Tool struct {
	// ID is a short, unique, name for this product to use internally, such as
	// for a filename.
	ID string `json:"id"`

	// Domain is a group of similar tooling functionality.
	Domain Domain `json:"domain"`

	// DisplayName of the tool in plain text.
	DisplayName string `json:"display_name"`

	// Description of the tool in plain text.
	Description string `json:"description"`

	// Phase of development that the tool is in.
	Phase Phase `json:"phase"`

	// TeamsID is the ID in the teams database.
	TeamsID string `json:"teams_id"`

	// CodePaths are links to where the code can be found.
	CodePaths []string `json:"code_path"`

	// Audience for the tool. That is, the pillars or groups of people that use
	// this tool.
	Audience []Audience `json:"audience"`

	// AdoptionStage for the tool in onboarding new users.
	AdoptionStage AcceptingCustomers `json:"adoption_stage"`

	// LandingPage URL.
	LandingPage string `json:"landing_page"`

	// Documentation maps a display name to URLs for documentation, FAQs, Getting Stated Guides, etc.
	Documentation map[string]string `json:"docs"`

	// Feedback maps a display name to URLs for providing feedback, such as Buganizer.
	Feedback map[string]string `json:"feedback"`

	// Resources maps a display name to URLs for resources that aren't either
	// Documentation or Feedback. For example, an announce-only email list, a
	// bug template to request a new instance, or a chat group.
	Resources map[string]string `json:"resources"`
}

// schema is a json schema for Tool, it is created by
// running go generate on ./generate/main.go.
//
//go:embed schema.json
var schema []byte

// FromJSON returns the deserialized JSON of a Tool.
//
// If the JSON did not conform to the schema then a list of schema violations
// may be returned also.
func FromJSON(ctx context.Context, jsonBody []byte) (*Tool, []string, error) {
	var tool Tool
	var schemaViolations []string = nil

	// Validate config here.
	schemaViolations, err := jsonschema.Validate(ctx, jsonBody, schema)
	if err != nil {
		return nil, schemaViolations, skerr.Wrapf(err, "validate Tool JSON")
	}
	err = json.Unmarshal(jsonBody, &tool)
	if err != nil {
		return nil, schemaViolations, skerr.Wrap(err)
	}

	return &tool, nil, nil
}

// LoadAndValidateFromFS loads the files from the given FS and also validates
// them at the same time.
func LoadAndValidateFromFS(ctx context.Context, fsys fs.FS) ([]*Tool, []string, error) {
	ret := []*Tool{}
	files, err := fs.Glob(fsys, "*.json")
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	if len(files) == 0 {
		sklog.Fatalf("Failed to find any config files.")
	}
	for _, filename := range files {
		b, err := fs.ReadFile(fsys, filename)
		if err != nil {
			return nil, nil, skerr.Wrapf(err, "Filename: %s", filename)
		}
		t, messages, err := FromJSON(ctx, b)
		if err != nil {
			return nil, messages, skerr.Wrapf(err, "Validation messages: %v", messages)
		}
		ret = append(ret, t)
	}
	return ret, nil, nil
}
