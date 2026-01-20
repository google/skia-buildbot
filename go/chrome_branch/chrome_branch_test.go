package chrome_branch

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/mockhttpclient"
)

const (
	fakeData = `[
	{
		"angle_branch": "4577",
		"bling_ldap": "govind",
		"bling_owner": "Krishna Govind",
		"chromium_branch": "4577",
		"chromium_main_branch_hash": "761ddde228655e313424edec06497d0c56b0f3c4",
		"chromium_main_branch_position": 902210,
		"clank_ldap": "benmason",
		"clank_owner": "Ben Mason",
		"cros_ldap": "geohsu",
		"cros_owner": "Geo Hsu",
		"dawn_branch": "4577",
		"desktop_ldap": "pbommana",
		"desktop_owner": "Prudhvi Bommana",
		"devtools_branch": "4577",
		"milestone": 93,
		"pdfium_branch": "4577",
		"schedule_active": true,
		"schedule_phase": "beta",
		"skia_branch": "m93",
		"v8_branch": "9.3-lkgr",
		"webrtc_branch": "4577"
	},
	{
		"angle_branch": "4515",
		"bling_ldap": "benmason",
		"bling_owner": "Ben Mason",
		"chromium_branch": "4515",
		"chromium_main_branch_hash": "488fc70865ddaa05324ac00a54a6eb783b4bc41c",
		"chromium_main_branch_position": 885287,
		"clank_ldap": "govind",
		"clank_owner": "Krishna Govind",
		"cros_ldap": "dgagnon",
		"cros_owner": "Daniel Gagnon",
		"dawn_branch": "4515",
		"desktop_ldap": "srinivassista",
		"desktop_owner": "Srinivas Sista",
		"devtools_branch": "4515",
		"milestone": 92,
		"pdfium_branch": "4515",
		"schedule_active": true,
		"schedule_phase": "stable",
		"skia_branch": "m92",
		"v8_branch": "9.2-lkgr",
		"webrtc_branch": "4515"
	},
	{
		"angle_branch": "4472",
		"bling_ldap": "bindusuvarna",
		"bling_owner": "Bindu Suvarna",
		"chromium_branch": "4472",
		"chromium_main_branch_hash": "3d60439cfb36485e76a1c5bb7f513d3721b20da1",
		"chromium_main_branch_position": 870763,
		"clank_ldap": "benmason",
		"clank_owner": "Ben Mason",
		"cros_ldap": "marinakz",
		"cros_owner": "Marina Kazatcker",
		"dawn_branch": null,
		"desktop_ldap": "pbommana",
		"desktop_owner": "Prudhvi Bommana",
		"devtools_branch": "4472",
		"milestone": 91,
		"pdfium_branch": "4472",
		"schedule_active": false,
		"skia_branch": "m91",
		"v8_branch": "9.1-lkgr",
		"webrtc_branch": "4472"
	}
]`

	fakeData2 = `[
	{
		"angle_branch": "4606",
		"bling_ldap": "harrysouders",
		"bling_owner": "Harry Souders",
		"chromium_branch": "4606",
		"chromium_main_branch_hash": "35b0d5a9dc8362adfd44e2614f0d5b7402ef63d0",
		"chromium_main_branch_position": 911515,
		"clank_ldap": "govind",
		"clank_owner": "Krishna Govind",
		"cros_ldap": "matthewjoseph",
		"cros_owner": "Matt Nelson",
		"dawn_branch": "4606",
		"desktop_ldap": "srinivassista",
		"desktop_owner": "Srinivas Sista",
		"devtools_branch": "4606",
		"milestone": 94,
		"pdfium_branch": "4606",
		"schedule_active": true,
		"schedule_phase": "branch",
		"skia_branch": "m94",
		"v8_branch": "9.4-lkgr",
		"webrtc_branch": "4606"
	},
	{
		"angle_branch": "4577",
		"bling_ldap": "govind",
		"bling_owner": "Krishna Govind",
		"chromium_branch": "4577",
		"chromium_main_branch_hash": "761ddde228655e313424edec06497d0c56b0f3c4",
		"chromium_main_branch_position": 902210,
		"clank_ldap": "benmason",
		"clank_owner": "Ben Mason",
		"cros_ldap": "geohsu",
		"cros_owner": "Geo Hsu",
		"dawn_branch": "4577",
		"desktop_ldap": "pbommana",
		"desktop_owner": "Prudhvi Bommana",
		"devtools_branch": "4577",
		"milestone": 93,
		"pdfium_branch": "4577",
		"schedule_active": true,
		"schedule_phase": "stable_cut",
		"skia_branch": "m93",
		"v8_branch": "9.3-lkgr",
		"webrtc_branch": "4577"
	},
	{
		"angle_branch": "4515",
		"bling_ldap": "benmason",
		"bling_owner": "Ben Mason",
		"chromium_branch": "4515",
		"chromium_main_branch_hash": "488fc70865ddaa05324ac00a54a6eb783b4bc41c",
		"chromium_main_branch_position": 885287,
		"clank_ldap": "govind",
		"clank_owner": "Krishna Govind",
		"cros_ldap": "dgagnon",
		"cros_owner": "Daniel Gagnon",
		"dawn_branch": "4515",
		"desktop_ldap": "srinivassista",
		"desktop_owner": "Srinivas Sista",
		"devtools_branch": "4515",
		"milestone": 92,
		"pdfium_branch": "4515",
		"schedule_active": true,
		"schedule_phase": "stable",
		"skia_branch": "m92",
		"v8_branch": "9.2-lkgr",
		"webrtc_branch": "4515"
	},
	{
		"angle_branch": "4472",
		"bling_ldap": "bindusuvarna",
		"bling_owner": "Bindu Suvarna",
		"chromium_branch": "4472",
		"chromium_main_branch_hash": "3d60439cfb36485e76a1c5bb7f513d3721b20da1",
		"chromium_main_branch_position": 870763,
		"clank_ldap": "benmason",
		"clank_owner": "Ben Mason",
		"cros_ldap": "marinakz",
		"cros_owner": "Marina Kazatcker",
		"dawn_branch": null,
		"desktop_ldap": "pbommana",
		"desktop_owner": "Prudhvi Bommana",
		"devtools_branch": "4472",
		"milestone": 91,
		"pdfium_branch": "4472",
		"schedule_active": false,
		"skia_branch": "m91",
		"v8_branch": "9.1-lkgr",
		"webrtc_branch": "4472"
	}
]`

	fakeData3 = `[
	{
		"angle_branch": "4606",
		"bling_ldap": "harrysouders",
		"bling_owner": "Harry Souders",
		"chromium_branch": "4606",
		"chromium_main_branch_hash": "35b0d5a9dc8362adfd44e2614f0d5b7402ef63d0",
		"chromium_main_branch_position": 911515,
		"clank_ldap": "govind",
		"clank_owner": "Krishna Govind",
		"cros_ldap": "matthewjoseph",
		"cros_owner": "Matt Nelson",
		"dawn_branch": "4606",
		"desktop_ldap": "srinivassista",
		"desktop_owner": "Srinivas Sista",
		"devtools_branch": "4606",
		"milestone": 94,
		"pdfium_branch": "4606",
		"schedule_active": true,
		"schedule_phase": "branch",
		"skia_branch": "m94",
		"v8_branch": "9.4-lkgr",
		"webrtc_branch": "4606"
	},
	{
		"angle_branch": "4577",
		"bling_ldap": "govind",
		"bling_owner": "Krishna Govind",
		"chromium_branch": "4577",
		"chromium_main_branch_hash": "761ddde228655e313424edec06497d0c56b0f3c4",
		"chromium_main_branch_position": 902210,
		"clank_ldap": "benmason",
		"clank_owner": "Ben Mason",
		"cros_ldap": "geohsu",
		"cros_owner": "Geo Hsu",
		"dawn_branch": "4577",
		"desktop_ldap": "pbommana",
		"desktop_owner": "Prudhvi Bommana",
		"devtools_branch": "4577",
		"milestone": 93,
		"pdfium_branch": "4577",
		"schedule_active": true,
		"schedule_phase": "beta",
		"skia_branch": "m93",
		"v8_branch": "9.3-lkgr",
		"webrtc_branch": "4577"
	},
	{
		"angle_branch": "4515",
		"bling_ldap": "benmason",
		"bling_owner": "Ben Mason",
		"chromium_branch": "4515",
		"chromium_main_branch_hash": "488fc70865ddaa05324ac00a54a6eb783b4bc41c",
		"chromium_main_branch_position": 885287,
		"clank_ldap": "govind",
		"clank_owner": "Krishna Govind",
		"cros_ldap": "dgagnon",
		"cros_owner": "Daniel Gagnon",
		"dawn_branch": "4515",
		"desktop_ldap": "srinivassista",
		"desktop_owner": "Srinivas Sista",
		"devtools_branch": "4515",
		"milestone": 92,
		"pdfium_branch": "4515",
		"schedule_active": true,
		"schedule_phase": "stable",
		"skia_branch": "m92",
		"v8_branch": "9.2-lkgr",
		"webrtc_branch": "4515"
	},
	{
		"angle_branch": "4472",
		"bling_ldap": "bindusuvarna",
		"bling_owner": "Bindu Suvarna",
		"chromium_branch": "4472",
		"chromium_main_branch_hash": "3d60439cfb36485e76a1c5bb7f513d3721b20da1",
		"chromium_main_branch_position": 870763,
		"clank_ldap": "benmason",
		"clank_owner": "Ben Mason",
		"cros_ldap": "marinakz",
		"cros_owner": "Marina Kazatcker",
		"dawn_branch": null,
		"desktop_ldap": "pbommana",
		"desktop_owner": "Prudhvi Bommana",
		"devtools_branch": "4472",
		"milestone": 91,
		"pdfium_branch": "4472",
		"schedule_active": true,
		"schedule_phase": "stable",
		"skia_branch": "m91",
		"v8_branch": "9.1-lkgr",
		"webrtc_branch": "4472"
	}
]`

	fakeData4 = `[
  {
    "angle_branch": "7632",
    "bling_ldap": null,
    "bling_owner": null,
    "chromium_branch": "7632",
    "chromium_main_branch_hash": "0bbdf2913883391365383b0a5dfe7bf9fd1a5213",
    "chromium_main_branch_position": 1568190,
    "clank_ldap": null,
    "clank_owner": null,
    "cros_ldap": "andywu",
    "cros_owner": "Andy Wu",
    "dawn_branch": "7632",
    "desktop_ldap": null,
    "desktop_ldap_emea": null,
    "desktop_ldap_us": "srinivassista",
    "desktop_owner": null,
    "desktop_owner_emea": null,
    "desktop_owner_us": "Srinivas Sista",
    "devtools_branch": "7632",
    "merge_phase": "low_priority",
    "milestone": 145,
    "mobile_ldap_emea": "eakpobaro",
    "mobile_ldap_us": "harrysouders",
    "mobile_owner_emea": "Erhu Akpobaro",
    "mobile_owner_us": "Harry Souders",
    "pdfium_branch": "7632",
    "schedule_active": true,
    "schedule_phase": "branch",
    "skia_branch": "m145",
    "v8_branch": "14.5",
    "webrtc_branch": "7632"
  },
  {
    "angle_branch": "7559",
    "bling_ldap": null,
    "bling_owner": null,
    "chromium_branch": "7559",
    "chromium_main_branch_hash": "223dfbac1c7542a06b422390d954afe5b560b607",
    "chromium_main_branch_position": 1552494,
    "clank_ldap": null,
    "clank_owner": null,
    "cros_ldap": "alonbajayo",
    "cros_owner": "Alon Bajayo",
    "dawn_branch": "7559",
    "desktop_ldap": null,
    "desktop_ldap_emea": null,
    "desktop_ldap_us": "srinivassista",
    "desktop_owner": null,
    "desktop_owner_emea": null,
    "desktop_owner_us": "Srinivas Sista",
    "devtools_branch": "7559",
    "merge_phase": "high_priority",
    "milestone": 144,
    "mobile_ldap_emea": "eakpobaro",
    "mobile_ldap_us": "govind",
    "mobile_owner_emea": "Erhu Akpobaro",
    "mobile_owner_us": "Krishna Govind",
    "pdfium_branch": "7559",
    "schedule_active": true,
    "schedule_phase": "stable",
    "skia_branch": "m144",
    "v8_branch": "14.4",
    "webrtc_branch": "7559"
  },
  {
    "angle_branch": "7499",
    "bling_ldap": null,
    "bling_owner": null,
    "chromium_branch": "7499",
    "chromium_main_branch_hash": "b30439823e5177773584139e72e0593e36863899",
    "chromium_main_branch_position": 1536371,
    "clank_ldap": null,
    "clank_owner": null,
    "cros_ldap": "lmenezes",
    "cros_owner": "Luis Menezes",
    "dawn_branch": "7499",
    "desktop_ldap": null,
    "desktop_ldap_emea": "danielyip",
    "desktop_ldap_us": "srinivassista",
    "desktop_owner": null,
    "desktop_owner_emea": "Daniel Yip",
    "desktop_owner_us": "Srinivas Sista",
    "devtools_branch": "7499",
    "merge_phase": "high_priority",
    "milestone": 143,
    "mobile_ldap_emea": "eakpobaro",
    "mobile_ldap_us": "harrysouders",
    "mobile_owner_emea": "Erhu Akpobaro",
    "mobile_owner_us": "Harry Souders",
    "pdfium_branch": "7499",
    "schedule_active": true,
    "schedule_phase": "stable",
    "skia_branch": "m143",
    "v8_branch": "14.3",
    "webrtc_branch": "7499"
  }
]`
)

func fakeBranches() *Branches {
	m := fakeMilestones()
	return &Branches{
		Main: &Branch{
			Milestone: 94,
			Number:    0,
			Ref:       RefMain,
			V8Branch:  RefMain,
		},
		Beta:   m[0],
		Stable: m[1],
	}
}

func fakeMilestones() []*Branch {
	return []*Branch{
		{
			Milestone: 93,
			Number:    4577,
			Ref:       fmt.Sprintf(refTmplRelease, 4577),
			V8Branch:  "9.3",
		},
		{
			Milestone: 92,
			Number:    4515,
			Ref:       fmt.Sprintf(refTmplRelease, 4515),
			V8Branch:  "9.2",
		},
	}
}

func TestBranchCopy(t *testing.T) {
	b := fakeBranches()
	assertdeep.Copy(t, b.Beta, b.Beta.Copy())
}

func TestBranchesCopy(t *testing.T) {
	b := fakeBranches()
	b.Main.Milestone = 95
	b.Dev = &Branch{
		Milestone: 94,
		Number:    4606,
		Ref:       fmt.Sprintf(refTmplRelease, 4606),
		V8Branch:  "9.4",
	}
	assertdeep.Copy(t, b, b.Copy())
}

func TestBranchValidate(t *testing.T) {
	test := func(fn func(*Branch), expectErr string) {
		b := fakeBranches().Beta
		fn(b)
		err := b.Validate()
		if expectErr == "" {
			require.NoError(t, err)
		} else {
			require.NotNil(t, err)
			require.True(t, strings.Contains(err.Error(), expectErr))
		}
	}

	// OK.
	test(func(b *Branch) {}, "")
	test(func(b *Branch) {
		b.Ref = RefMain
		b.Number = 0
	}, "")

	// Not OK.
	test(func(b *Branch) {
		b.Milestone = 0
	}, "Milestone is required")
	test(func(b *Branch) {
		b.Number = 0
	}, "Number is required")
	test(func(b *Branch) {
		b.Ref = RefMain
	}, "Number must be zero for main branch")
}

func TestBranchesValidate(t *testing.T) {
	test := func(fn func(*Branches), expectErr string) {
		b := fakeBranches()
		fn(b)
		err := b.Validate()
		if expectErr == "" {
			require.NoError(t, err)
		} else {
			require.NotNil(t, err)
			require.True(t, strings.Contains(err.Error(), expectErr), err)
		}
	}

	// OK.
	test(func(b *Branches) {}, "")

	// Missing branch.
	test(func(b *Branches) {
		b.Beta = nil
	}, "Beta branch is missing")
	test(func(b *Branches) {
		b.Stable = nil
	}, "Stable branch is missing")
	test(func(b *Branches) {
		b.Main = nil
	}, "Main branch is missing")

	// Each Branch should be validated.
	test(func(b *Branches) {
		b.Beta.Milestone = 0
	}, "Milestone is required")
	test(func(b *Branches) {
		b.Stable.Number = 0
	}, "Number is required")
	test(func(b *Branches) {
		b.Main.Number = 42
	}, "Number must be zero for main branch.")
}

func TestGet(t *testing.T) {
	ctx := context.Background()
	urlmock := mockhttpclient.NewURLMock()
	c := urlmock.Client()

	// Everything okay.
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(fakeData)))
	b, m, err := Get(ctx, c)
	require.NoError(t, err)
	require.Equal(t, fakeBranches(), b)
	require.Equal(t, fakeMilestones(), m)

	// Beta channel is missing, we retrieve the branch via milestone number.
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(strings.ReplaceAll(fakeData, schedulePhaseBeta, "dev"))))
	b, m, err = Get(ctx, c)
	require.NoError(t, err)
	require.Equal(t, fakeBranches(), b)
	require.Equal(t, fakeMilestones(), m)

	// Beta channel is actually missing.
	noBeta := strings.ReplaceAll(strings.ReplaceAll(fakeData, schedulePhaseBeta, "dev"), strconv.Itoa(fakeBranches().Beta.Milestone), "9999")
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(noBeta)))
	b, m, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "Beta branch is missing"), err)

	// Stable channel is missing.
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(strings.ReplaceAll(fakeData, schedulePhaseStable, "dev"))))
	b, m, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "Stable branch is missing"), err)

	// Invalid branch number.
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(strings.ReplaceAll(fakeData, "4577", "nope"))))
	b, m, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "invalid branch number \"nope\" for channel \"beta\""), err)

	// Missing milestone.
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(strings.ReplaceAll(fakeData, "93", "null"))))
	b, m, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "Beta branch is invalid: Milestone is required"), err)
}

func TestGetSecondDataSet(t *testing.T) {
	ctx := context.Background()
	urlmock := mockhttpclient.NewURLMock()
	c := urlmock.Client()

	// Everything okay. This data set is missing the "beta" branch in
	// schedule_phase, so we fall back to using "stable_cut".
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(fakeData2)))
	b, _, err := Get(ctx, c)
	require.NoError(t, err)
	expect := fakeBranches()
	expect.Main.Milestone = 95
	expect.Dev = &Branch{
		Milestone: 94,
		Number:    4606,
		Ref:       fmt.Sprintf(refTmplRelease, 4606),
		V8Branch:  "9.4",
	}
	require.Equal(t, expect, b)

	// "stable_cut" is missing, so we fall back to using "branch"
	noBeta := strings.ReplaceAll(strings.ReplaceAll(fakeData2, schedulePhaseStableCut, "other"), strconv.Itoa(fakeBranches().Beta.Milestone), "9999")
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(noBeta)))
	b, _, err = Get(ctx, c)
	require.NoError(t, err)
	expect.Beta = expect.Dev
	expect.Dev = nil

	// Stable channel is missing.
	noStable := strings.ReplaceAll(fakeData2, fmt.Sprintf("\"%s\"", schedulePhaseStable), "\"dev\"")
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(noStable)))
	b, _, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "Stable branch is missing"), err)

	// Invalid branch number.
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(strings.ReplaceAll(fakeData2, "4577", "nope"))))
	b, _, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "invalid branch number \"nope\" for channel \"stable_cut\""), err)

	// Missing milestone.
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(strings.ReplaceAll(fakeData2, "93", "null"))))
	b, _, err = Get(ctx, c)
	require.Nil(t, b)
	require.NotNil(t, err)
	require.True(t, strings.Contains(err.Error(), "Beta branch is invalid: Milestone is required"), err)
}

func TestGetThirdDataSet(t *testing.T) {
	ctx := context.Background()
	urlmock := mockhttpclient.NewURLMock()
	c := urlmock.Client()

	// Everything okay. This data set has two "stable" branches in
	// schedule_phase, so we use the highest milestone number.
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(fakeData3)))
	b, _, err := Get(ctx, c)
	require.NoError(t, err)
	expect := fakeBranches()
	expect.Main.Milestone = 95
	expect.Dev = &Branch{
		Milestone: 94,
		Number:    4606,
		Ref:       fmt.Sprintf(refTmplRelease, 4606),
		V8Branch:  "9.4",
	}
	require.Equal(t, expect, b)
}

func TestGetFourthDataSet(t *testing.T) {
	ctx := context.Background()
	urlmock := mockhttpclient.NewURLMock()
	c := urlmock.Client()

	// Everything okay. This data set has no "beta" branch; instead there's a
	// "branch". Ensure that Beta falls back to using that.
	urlmock.MockOnce(jsonURL, mockhttpclient.MockGetDialogue([]byte(fakeData4)))
	b, _, err := Get(ctx, c)
	require.NoError(t, err)
	expect := &Branches{
		Main: &Branch{
			Milestone: 146,
			Ref:       "refs/heads/main",
			V8Branch:  "refs/heads/main",
		},
		Beta: &Branch{
			Milestone: 145,
			Number:    7632,
			Ref:       fmt.Sprintf(refTmplRelease, 7632),
			V8Branch:  "14.5",
		},
		Dev: &Branch{
			Milestone: 145,
			Number:    7632,
			Ref:       fmt.Sprintf(refTmplRelease, 7632),
			V8Branch:  "14.5",
		},
		Stable: &Branch{
			Milestone: 144,
			Number:    7559,
			Ref:       "refs/branch-heads/7559",
			V8Branch:  "14.4",
		},
	}
	require.Equal(t, expect, b)
}
