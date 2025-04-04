package datakitchensink

import (
	"encoding/hex"
	"path/filepath"
	"time"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/golden/go/sql/databuilder"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

// Build creates a set of data that covers many common testing scenarios.
func Build() schema.Tables {
	base := RawBuilder()
	return base.Build()
}

// RawBuilder returns the builder underlying Build(). It can be useful for tests to add a small
// tweak to this existing data and then rebuild (For example, in tests where there doesn't need to
// be any new digests).
func RawBuilder() databuilder.TablesBuilder {
	b := databuilder.TablesBuilder{TileWidth: 5}
	// This data set has data on 10 commits and no data on 3 commits in the middle.
	// Intentionally put these commits such that they straddle a tile.
	// Commits with ids 103-105 have no data to help test sparse data.
	b.CommitsWithData().
		Insert(OldestCommitID, UserOne, "commit 98", "2020-12-01T00:00:00Z").
		Insert("0000000099", UserTwo, "commit 99", "2020-12-02T00:00:00Z").
		Insert("0000000100", UserThree, "commit 100", "2020-12-03T00:00:00Z").
		Insert(WindowsDriverUpdateCommitID, UserTwo, "Update Windows 10.2 to 10.3", "2020-12-04T00:00:00Z").
		Insert("0000000102", UserOne, "commit 102", "2020-12-05T00:00:00Z").
		Insert("0000000106", UserTwo, "Add walleye device", "2020-12-07T00:00:00Z").
		Insert("0000000107", UserThree, "Add taimen device [flaky]", "2020-12-08T00:00:00Z").
		Insert(IOSFixTriangleTestsBreakCircleTestsCommitID, UserTwo, "Fix iOS Triangle tests [accidental break of circle tests]", "2020-12-09T00:00:00Z").
		Insert("0000000109", UserOne, "Enable autotriage of walleye", "2020-12-10T00:00:00Z").
		Insert(MostRecentCommitID, UserTwo, "commit 110", "2020-12-11T00:00:00Z")

	b.CommitsWithNoData().
		Insert("0000000103", "0103010301030103010301030103010301030103", UserFour, "no data 103", "2020-12-06T01:00:00Z").
		Insert("0000000104", "0104010401040104010401040104010401040104", UserFour, "no data 104", "2020-12-06T02:00:00Z").
		Insert("0000000105", "0105010501050105010501050105010501050105", UserFour, "no data 105", "2020-12-06T03:00:00Z").
		// This commit has the highest commit id, out of the ones listed here and above. We use an actual commit hash
		// because most recent commit gets "special treatment" in the gitilesfollower code. Having a non-existent commit
		// hash results in a crash.
		Insert("0000000111", "acb4c0deaa115916044699809a363d539cf725da", UserFour, "no data 111", "2020-12-12T12:00:00Z")

	b.SetDigests(map[rune]types.Digest{
		// by convention, upper case are positively triaged, lowercase
		// are untriaged, numbers are negative, symbols are special.
		'A': DigestA01Pos,
		'B': DigestA02Pos,
		'C': DigestA03Pos,
		'd': DigestA04Unt,
		'e': DigestA05Unt,
		'f': DigestA06Unt,
		'G': DigestA07Pos,
		'H': DigestA08Pos,
		'1': DigestA09Neg,

		'K': DigestB01Pos,
		'L': DigestB02Pos,
		'3': DigestB03Neg,
		'4': DigestB04Neg,
		'M': DigestB05Pos_CL,

		'P': DigestC01Pos,
		'Q': DigestC02Pos,
		'r': DigestC03Unt,
		's': DigestC04Unt,
		't': DigestC05Unt,
		'U': DigestC06Pos_CL,
		'v': DigestC07Unt_CL,

		'W': DigestD01Pos_CL,

		'X': DigestE01Pos_CL,
		'Y': DigestE02Pos_CL,
		'Z': DigestE03Unt_CL,
		// This digest is for a blank image, which has been triaged as negative on the circle test,
		// but not the others. When it shows up in other groupings, it should be untriaged
		'@': DigestBlank,
	})
	b.SetGroupingKeys(types.CorpusField, types.PrimaryKeyField)

	// The windows machines were upgraded from 10.2 to 10.3 after the 3rd commit
	b.AddTracesWithCommonKeys(paramtools.Params{
		OSKey:     Windows10dot2OS,
		DeviceKey: QuadroDevice,
	}).History(
		"AAA-------",
		"@KK-------", // This trace is a little non-deterministic
		"PPP-------",
		"BBC-------", // This trace is a little non-deterministic
		"LLL-------",
		"QQQ-------",
	).Keys([]paramtools.Params{
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{WindowsFile1, WindowsFile2, WindowsFile3, "", "", "", "", "", "", ""},
			[]string{"2020-12-01T00:42:00Z", "2020-12-02T00:43:00Z", "2020-12-03T00:44:00Z", "", "", "", "", "", "", ""})

	b.AddTracesWithCommonKeys(paramtools.Params{
		OSKey:     Windows10dot3OS,
		DeviceKey: QuadroDevice,
	}).History(
		"---AAAAAAA",
		"---KKKKKKK", // 10.3 had a bugfix for the flaky behavior
		"---rrrrrrr", // The new driver in 10.3 changed the antialias for circles a little.
		"---BCBBCB-", // This trace is still a little non-deterministic. The grey tests are
		"---LLLLLL-", // still running and haven't been ingested yet.
		"---ssssss-",
	).Keys([]paramtools.Params{
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{"", "", "",
			WindowsFile4, WindowsFile5, WindowsFile6, WindowsFile7, WindowsFile8, WindowsFile9, WindowsFile10},
			[]string{"", "", "", "2020-12-04T00:45:00Z", "2020-12-05T00:46:00Z", "2020-12-07T00:31:00Z",
				"2020-12-08T00:32:00Z", "2020-12-09T00:33:00Z", "2020-12-10T00:34:00Z", "2020-12-11T00:35:00Z"})

	// iPad traces had a bug fix and a bug introduced at commit 7.
	b.AddTracesWithCommonKeys(paramtools.Params{
		OSKey:     IOS,
		DeviceKey: IPadDevice,
	}).History(
		"AAAAAAAAAA",
		"33@33@3KKK", // This trace was drawing incorrectly until commit index 7.
		// This trace was drawing correctly until commit index 7. Some data is missing due to slow
		// or flaky tests.
		"PPPPP----t",
		"BCBCBBBBdC", // This trace is a little non-deterministic
		"444@444LLL", // This trace was drawing incorrectly until commit index 7.
		// This trace was drawing correctly until commit index 7. Some data is missing due to slow
		// or flaky tests.
		"QQ------t-",
	).Keys([]paramtools.Params{
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{IpadFile1, IpadFile2, IpadFile3, IpadFile4, IpadFile5, IpadFile6, IpadFile7, IpadFile8, IpadFile9, IpadFile10},
			[]string{"2020-12-01T00:31:00Z", "2020-12-02T00:31:00Z", "2020-12-03T00:31:00Z", "2020-12-04T00:31:00Z",
				"2020-12-05T00:31:00Z", "2020-12-07T00:31:00Z", "2020-12-08T00:31:00Z", "2020-12-09T00:31:00Z",
				"2020-12-10T00:31:00Z", "2020-12-11T00:31:00Z"})

	// The Iphones are meant to draw the same as the iPads, however we pretend the iPhone tests
	// are slow and thus have sparse data. We do this by saying the RGB data is missing on every
	// other commit and the GREY data is missing on two commits out of three.
	b.AddTracesWithCommonKeys(paramtools.Params{
		OSKey:        IOS,
		DeviceKey:    IPhoneDevice,
		ColorModeKey: RGBColorMode,
	}).History(
		"A-A-A-A-A-",
		"3-@-@-3-K-", // This trace was drawing incorrectly until commit index 7.
		"P-P-----t-", // This trace was drawing correctly until commit index 7. Slower tests.
	).Keys([]paramtools.Params{
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{IPhoneFile1a, "", IPhoneFile3a, "", IPhoneFile5a, "", IPhoneFile7a, "", IPhoneFile9a, ""},
			[]string{"2020-12-01T7:31:00Z", "", "2020-12-03T07:31:00Z", "", "2020-12-05T07:31:00Z",
				"", "2020-12-08T07:31:00Z", "", "2020-12-10T07:31:00Z", ""})

	b.AddTracesWithCommonKeys(paramtools.Params{
		OSKey:        IOS,
		DeviceKey:    IPhoneDevice,
		ColorModeKey: GreyColorMode,
	}).History(
		"-B--B--B--",
		"-@--4--L--", // This trace was drawing incorrectly until commit index 7.
		"-Q-----t--", // This trace was drawing correctly until commit index 7. Slower tests.
	).Keys([]paramtools.Params{
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{"", IPhoneFile2b, "", "", IPhoneFile5b, "", "", IPhoneFile8b, "", ""},
			[]string{"", "2020-12-02T7:18:00Z", "", "", "2020-12-05T7:18:00Z", "", "", "2020-12-09T7:18:00Z", "", ""})

	// The walleye device wasn't being tested until commit index 5. One trace was really flaky, so
	// it was configured to use fuzzy matching at commit index 8.
	walleyeFiles := []string{"", "", "", "", "", WalleyeFile6, WalleyeFile7, WalleyeFile8, WalleyeFile9, WalleyeFile10}
	walleyeTimes := []string{"", "", "", "", "", "2020-12-07T00:21:00Z", "2020-12-08T00:21:00Z", "2020-12-09T00:21:00Z",
		"2020-12-10T00:21:00Z", "2020-12-11T00:21:00Z"}
	walleyeFuzzyParams := paramtools.Params{
		"ext":                        "png",
		"image_matching_algorithm":   "fuzzy",
		"fuzzy_max_different_pixels": "2"}
	b.AddTracesWithCommonKeys(paramtools.Params{
		OSKey:     AndroidOS,
		DeviceKey: WalleyeDevice,
	}).History(
		"-----KKKKK",
		"-----PPPPP",
		"-----BB-BB", // At commit index 7, the GREY data was only partially reported.
		"-----LL-LL",
		"-----QQ-QQ",
	).Keys([]paramtools.Params{
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom(walleyeFiles, walleyeTimes)
	b.AddTracesWithCommonKeys(paramtools.Params{
		OSKey:             AndroidOS,
		DeviceKey:         WalleyeDevice,
		ColorModeKey:      RGBColorMode,
		types.CorpusField: CornersCorpus,
	}).History("-----eAfGH").Keys([]paramtools.Params{{types.PrimaryKeyField: SquareTest}}).
		OptionsPerPoint([][]paramtools.Params{{nil, nil, nil, nil, nil,
			{"ext": "png"}, {"ext": "png"}, {"ext": "png"}, walleyeFuzzyParams, walleyeFuzzyParams}}).
		IngestedFrom(walleyeFiles, walleyeTimes)

	// The taimen device was added in commit index 6. It only runs the RGB Configs. 2 out of the 3
	// traces are ignored. The non-ignored trace uses the "positive_if_only_image" image matching
	// algorithm, and has triaging disallowed via the "disallow_triaging" optional key.
	b.AddTracesWithCommonKeys(paramtools.Params{
		OSKey:        AndroidOS,
		DeviceKey:    TaimenDevice,
		ColorModeKey: RGBColorMode,
	}).History(
		"------11A1", // This trace should be ignored
		"------KKKK",
		"------tttt", // This trace should be ignored
	).Keys([]paramtools.Params{
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}}).
		OptionsPerTrace([]paramtools.Params{
			{"ext": "png"},
			{"ext": "png", "image_matching_algorithm": "positive_if_only_image", "disallow_triaging": "true"},
			{"ext": "png"},
		}).
		IngestedFrom([]string{"", "", "", "", "", "", TaimenFile7, TaimenFile8, TaimenFile9, TaimenFile10},
			[]string{"", "", "", "", "", "", "2020-12-08T00:19:00Z", "2020-12-09T00:19:00Z", "2020-12-10T00:19:00Z", "2020-12-11T00:19:00Z"})

	b.AddTriageEvent(UserOne, "2020-06-07T08:09:10Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}).
		Positive(DigestC01Pos).Positive(DigestC02Pos)
	b.AddTriageEvent(UserOne, "2020-06-07T08:09:43Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest}).
		Positive(DigestB01Pos).Positive(DigestB02Pos)
	b.AddTriageEvent(UserTwo, "2020-06-07T08:15:04Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest}).
		Negative(DigestB03Neg)
	// Accidentally triaged to the wrong state.
	b.AddTriageEvent(UserTwo, "2020-06-07T08:15:07Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest}).
		Positive(DigestB04Neg)
	// Fixed it.
	b.AddTriageEvent(UserTwo, "2020-06-07T08:15:08Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest}).
		Triage(DigestB04Neg, schema.LabelPositive, schema.LabelNegative)
	b.AddTriageEvent(UserOne, "2020-06-07T08:23:08Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}).
		Positive(DigestA01Pos).Positive(DigestA02Pos)
	b.AddTriageEvent(UserThree, "2020-06-11T12:13:14Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}).
		Positive(DigestA03Pos)
	b.AddTriageEvent(UserThree, "2020-06-11T12:13:15Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}).
		Negative(DigestBlank)

	b.AddTriageEvent(UserThree, "2020-12-10T10:10:10Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}).
		Positive(DigestA07Pos)
	b.AddTriageEvent(AutoTriageUser, "2020-12-11T11:11:00Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}).
		Positive(DigestA08Pos)

	b.AddTriageEvent(UserFour, "2020-12-11T13:00:00Z").
		ExpectationsForGrouping(map[string]string{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}).
		Negative(DigestA09Neg)

	b.AddIgnoreRule(UserTwo, UserOne, "2030-12-30T15:16:17Z", "Taimen isn't drawing correctly enough yet",
		paramtools.ParamSet{
			DeviceKey:             []string{TaimenDevice},
			types.PrimaryKeyField: []string{SquareTest, CircleTest},
		})
	b.AddIgnoreRule(UserTwo, UserOne, "2020-02-14T13:12:11Z", "This rule has expired (and does not apply to anything)",
		paramtools.ParamSet{
			DeviceKey:         []string{"Nokia4"},
			types.CorpusField: []string{CornersCorpus},
		})

	// This changelist has one patchset that adds some data which corrects the iOS glitch on the
	// iPads, but not for the iPhones.
	cl := b.AddChangelist(ChangelistIDThatAttemptsToFixIOS, GerritCRS, UserOne, "Fix iOS", schema.StatusOpen)
	ps := cl.AddPatchset(PatchSetIDFixesIPadButNotIPhone, "ffff111111111111111111111111111111111111", 3)
	ps.DataWithCommonKeys(paramtools.Params{
		OSKey: IOS, DeviceKey: IPhoneDevice, ColorModeKey: RGBColorMode,
	}).Digests(DigestA01Pos, // same as primary branch
		DigestB01Pos,    // same as primary branch
		DigestC07Unt_CL, // Newly seen digest (still not correct).
	).Keys([]paramtools.Params{
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
	}).OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob01IPhoneRGB, BuildBucketCIS, "Test-iPhone-RGB", Tryjob01FileIPhoneRGB, "2020-12-10T04:05:06Z")
	ps.DataWithCommonKeys(paramtools.Params{
		OSKey: IOS, DeviceKey: IPadDevice,
	}).Digests(DigestA01Pos, // same as primary branch
		DigestB01Pos,    // on this CL, the digest has been (incorrectly) marked as untriaged.
		DigestC06Pos_CL, // not on primary branch, triaged on CL.
		DigestA02Pos,    // same as primary branch
		DigestB02Pos,    // same as primary branch
		DigestC02Pos).   // Now correct (primary branch is producing untriaged).
		Keys([]paramtools.Params{
			{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest, ColorModeKey: RGBColorMode},
			{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest, ColorModeKey: RGBColorMode},
			{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest, ColorModeKey: RGBColorMode},
			{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest, ColorModeKey: GreyColorMode},
			{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest, ColorModeKey: GreyColorMode},
			{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest, ColorModeKey: GreyColorMode},
		}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob02IPad, BuildBucketCIS, "Test-iPad-ALL", Tryjob02FileIPad, "2020-12-10T03:02:01Z")
	ps.DataWithCommonKeys(paramtools.Params{
		OSKey: AndroidOS, DeviceKey: TaimenDevice, ColorModeKey: RGBColorMode,
	}).Digests(DigestA09Neg, // On primary branch, should be ignored.
		DigestB01Pos, // on this CL, the digest has been (incorrectly) marked as untriaged.
		DigestC05Unt, // On primary branch, should be ignored.
	).Keys([]paramtools.Params{
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
	}).OptionsPerPoint([]paramtools.Params{
		{"ext": "png"},
		{"ext": "png", "disallow_triaging": "true", "image_matching_algorithm": "positive_if_only_image"},
		{"ext": "png"},
	}).FromTryjob(Tryjob03TaimenRGB, BuildBucketCIS, "Test-taimen-RGB", Tryjob03FileTaimenRGB, "2020-12-10T03:44:44Z")
	cl.AddTriageEvent(UserOne, "2020-12-10T05:00:00Z").
		ExpectationsForGrouping(paramtools.Params{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest}).
		Triage(DigestB01Pos, schema.LabelPositive, schema.LabelUntriaged) // accidental triage
	cl.AddTriageEvent(UserOne, "2020-12-10T05:00:02Z").
		ExpectationsForGrouping(paramtools.Params{types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest}).
		Positive(DigestC06Pos_CL)

	// This CL adds some new tests over two patchsets. Additionally, this CL has data coming in
	// from an internal CRS and CIS.
	cl = b.AddChangelist(ChangelistIDThatAddsNewTests, GerritInternalCRS, UserTwo, "Increase test coverage", schema.StatusOpen)
	ps1 := cl.AddPatchset(PatchsetIDAddsNewCorpus, "eeee222222222222222222222222222222222222", 1)
	ps2 := cl.AddPatchset(PatchsetIDAddsNewCorpusAndTest, "eeee333333333333333333333333333333333333", 4)
	// Oops, the first PS adds a new corpus (containing one test), but the output is all blank.
	// All other data is what was drawn at head.
	ps1.DataWithCommonKeys(paramtools.Params{
		OSKey:     Windows10dot3OS,
		DeviceKey: QuadroDevice,
	}).Digests(DigestA01Pos, DigestB01Pos, DigestC03Unt, DigestBlank,
		DigestA03Pos, DigestB02Pos, DigestC04Unt, DigestBlank).
		Keys([]paramtools.Params{
			{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
			{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
			{ColorModeKey: RGBColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
			{ColorModeKey: RGBColorMode, types.CorpusField: TextCorpus, types.PrimaryKeyField: SevenTest},
			{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
			{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
			{ColorModeKey: GreyColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
			{ColorModeKey: GreyColorMode, types.CorpusField: TextCorpus, types.PrimaryKeyField: SevenTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob04Windows, BuildBucketInternalCIS, "Test-Windows10.3-ALL", Tryjob04FileWindows, "2020-12-12T08:09:10Z")
	// The second PS fixes the text corpus test and adds a round rect test to the existing
	// round corpus. Windows draws the new RoundRect test fine, but not the walleye device.
	ps2.DataWithCommonKeys(paramtools.Params{
		OSKey:     Windows10dot3OS,
		DeviceKey: QuadroDevice,
	}).Digests(DigestA01Pos, DigestB01Pos, DigestC03Unt,
		DigestE01Pos_CL, // Windows draws RoundRect test RGB correctly
		DigestD01Pos_CL, // Windows draws Text test correctly (intentionally the same as GREY)
		DigestA03Pos, DigestB02Pos, DigestC04Unt,
		DigestE02Pos_CL, // Windows draws RoundRect test GREY correctly
		DigestD01Pos_CL, // Windows draws Text test correctly (intentionally the same as RGB)
	).Keys([]paramtools.Params{
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: RoundRectTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: TextCorpus, types.PrimaryKeyField: SevenTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: RoundRectTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: TextCorpus, types.PrimaryKeyField: SevenTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob05Windows, BuildBucketInternalCIS, "Test-Windows10.3-ALL", Tryjob05FileWindows, "2020-12-12T09:00:00Z")
	// Data from the walleye is the same as head, except it draws the RoundRect incorrectly in RGB.
	ps2.DataWithCommonKeys(paramtools.Params{
		OSKey:     AndroidOS,
		DeviceKey: WalleyeDevice,
	}).Digests(DigestA07Pos, DigestB01Pos, DigestC01Pos,
		DigestE03Unt_CL, // Windows draws RoundRect test RGB wrong.
		DigestD01Pos_CL, DigestA02Pos, DigestB02Pos, DigestC02Pos,
		DigestE02Pos_CL, // Windows draws RoundRect test GREY correctly
		DigestD01Pos_CL, // Walleye draws Text test correctly (intentionally the same as RGB)
	).Keys([]paramtools.Params{
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: RoundRectTest},
		{ColorModeKey: RGBColorMode, types.CorpusField: TextCorpus, types.PrimaryKeyField: SevenTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: CircleTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: RoundCorpus, types.PrimaryKeyField: RoundRectTest},
		{ColorModeKey: GreyColorMode, types.CorpusField: TextCorpus, types.PrimaryKeyField: SevenTest}}).
		OptionsPerPoint([]paramtools.Params{
			walleyeFuzzyParams, {"ext": "png"}, {"ext": "png"}, {"ext": "png"}, {"ext": "png"},
			{"ext": "png"}, {"ext": "png"}, {"ext": "png"}, {"ext": "png"}, {"ext": "png"},
		}).
		FromTryjob(Tryjob06Walleye, BuildBucketInternalCIS, "Test-Walleye-ALL", Tryjob06FileWalleye, "2020-12-12T09:20:33Z")

	cl.AddTriageEvent(UserTwo, "2020-12-12T09:30:00Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: RoundCorpus, types.PrimaryKeyField: RoundRectTest,
		}).Positive(DigestE01Pos_CL).Positive(DigestE02Pos_CL)
	cl.AddTriageEvent(UserFour, "2020-12-12T09:30:12Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: TextCorpus, types.PrimaryKeyField: SevenTest,
		}).Positive(DigestD01Pos_CL)
	// Another accidental triage
	cl.AddTriageEvent(UserTwo, "2020-12-12T09:31:19Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: RoundCorpus, types.PrimaryKeyField: RoundRectTest,
		}).Negative(DigestE03Unt_CL)
	// Triage it correctly now.
	cl.AddTriageEvent(UserTwo, "2020-12-12T09:31:32Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: RoundCorpus, types.PrimaryKeyField: RoundRectTest,
		}).Triage(DigestE03Unt_CL, schema.LabelNegative, schema.LabelUntriaged)

	// Add two CLs that have a small amount of data, but are landed and/or abandoned
	cl = b.AddChangelist(ChangelistIDThatHasLanded, GerritCRS, UserTwo, "was landed", schema.StatusLanded)
	ps = cl.AddPatchset(PatchsetIDIsLanded, "aaaaa11111111111111111111111111111111111", 1)
	ps.DataWithCommonKeys(paramtools.Params{
		OSKey:     Windows10dot3OS,
		DeviceKey: QuadroDevice,
	}).Digests(DigestA01Pos).
		Keys([]paramtools.Params{
			{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob07Windows, BuildBucketCIS, "Test-Windows10.3-Some", Tryjob07FileWindows, "2020-05-05T05:05:00Z")

	cl = b.AddChangelist(ChangelistIDThatIsAbandoned, GerritCRS, UserOne, "was abandoned", schema.StatusAbandoned)
	ps = cl.AddPatchset(PatchsetIDIsAbandoned, "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb55555", 1)
	ps.DataWithCommonKeys(paramtools.Params{
		OSKey:     Windows10dot3OS,
		DeviceKey: QuadroDevice,
	}).Digests(DigestA04Unt).
		Keys([]paramtools.Params{
			{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob08Windows, BuildBucketCIS, "Test-Windows10.3-Some", Tryjob08FileWindows, "2020-06-06T06:06:00Z")

	// Add a CL that has multiple datapoints at the same patchset for the same trace. In this CL,
	// a tryjob was executed multiple times at the same patchset in order to investigate a flaky
	// test.
	cl = b.AddChangelist(ChangelistIDWithMultipleDatapointsPerTrace, GerritCRS, UserOne, "multiple datapoints", schema.StatusOpen)
	ps = cl.AddPatchset(PatchsetIDWithMultipleDatapointsPerTrace, "ccccccccccccccccccccccccccccccccccc66666", 1)
	ps.DataWithCommonKeys(paramtools.Params{
		OSKey:     Windows10dot3OS,
		DeviceKey: QuadroDevice,
	}).
		Digests(DigestC03Unt). // This CL/PS makes SquareTest erroneously draw circley things.
		Keys([]paramtools.Params{
			{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob09Windows, BuildBucketCIS, "Test-Windows10.3-Some", Tryjob09FileWindows, "2020-12-12T10:00:00Z").
		Digests(DigestC04Unt). // This CL/PS also makes SquareTest flaky.
		Keys([]paramtools.Params{
			{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob10Windows, BuildBucketCIS, "Test-Windows10.3-Some", Tryjob10FileWindows, "2020-12-12T11:00:00Z").
		Digests(DigestC03Unt). // The third run produces the same digest as the first run.
		Keys([]paramtools.Params{
			{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob11Windows, BuildBucketCIS, "Test-Windows10.3-Some", Tryjob11FileWindows, "2020-12-12T12:00:00Z").
		Digests(DigestC01Pos). // Produces a new digest which we accidentally triaged positive.
		Keys([]paramtools.Params{
			{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob12Windows, BuildBucketCIS, "Test-Windows10.3-Some", Tryjob12FileWindows, "2020-12-12T13:00:00Z").
		Digests(DigestC01Pos). // Same digest as previous run.
		Keys([]paramtools.Params{
			{ColorModeKey: RGBColorMode, types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest}}).
		OptionsAll(paramtools.Params{"ext": "png"}).
		FromTryjob(Tryjob13Windows, BuildBucketCIS, "Test-Windows10.3-Some", Tryjob13FileWindows, "2020-12-12T14:00:00Z")
	// Accidental triage.
	cl.AddTriageEvent(UserOne, "2020-12-12T15:00:00Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: CornersCorpus, types.PrimaryKeyField: SquareTest,
		}).Positive(DigestC01Pos)

	// Add a CL that defines a new test with disallow_triaging=true.
	cl = b.AddChangelist(ChangelistIDWithDisallowTriagingTest, GerritCRS, UserOne, "add test with disallow triaging", schema.StatusOpen)
	ps = cl.AddPatchset(PatchsetIDWithDisallowTriagingTest, "ddddddddddddddddddddddddddddddddddd77777", 1)
	ps.DataWithCommonKeys(paramtools.Params{
		OSKey: AndroidOS, DeviceKey: TaimenDevice, ColorModeKey: GreyColorMode,
	}).Digests(DigestB05Pos_CL).
		Keys([]paramtools.Params{
			{types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest},
		}).OptionsAll(paramtools.Params{
		"ext":                      "png",
		"disallow_triaging":        "true",
		"image_matching_algorithm": "positive_if_only_image",
	}).FromTryjob(Tryjob14TaimenGrey, BuildBucketCIS, "Test-taimen-Grey", Tryjob14FileTaimenGrey, "2020-12-12T16:00:00Z")
	// The digest was automatically triaged as positive by goldctl due to the positive_if_only_image
	// image matching algorithm.
	cl.AddTriageEvent(UserOne, "2020-12-12T17:00:00Z").
		ExpectationsForGrouping(paramtools.Params{
			types.CorpusField: CornersCorpus, types.PrimaryKeyField: TriangleTest,
		}).Positive(DigestB05Pos_CL)

	b.ComputeDiffMetricsFromImages(GetImgDirectory(), "2020-12-12T12:12:12Z")
	return b
}

// GetImgDirectory returns the path to the img directory in this folder that is friendly to both
// `go test` and `bazel test`.
func GetImgDirectory() string {
	root, err := repo_root.Get()
	if err != nil {
		panic(err)
	}
	return filepath.Join(root, "golden", "go", "sql", "datakitchensink", "img")
}

const (
	// Digests starting with an "a" belong to the square tests, a "b" prefix is for triangle, and so
	// on. The numbers (and thus the hash itself) are arbitrary. The suffix reveals how these are
	// triaged as of the last commit.
	// Square Images
	DigestA01Pos = types.Digest("a01a01a01a01a01a01a01a01a01a01a0")
	DigestA02Pos = types.Digest("a02a02a02a02a02a02a02a02a02a02a0") // GREY version of A01
	DigestA03Pos = types.Digest("a03a03a03a03a03a03a03a03a03a03a0") // small diff from A02
	DigestA04Unt = types.Digest("a04a04a04a04a04a04a04a04a04a04a0") // small diff from A02
	DigestA05Unt = types.Digest("a05a05a05a05a05a05a05a05a05a05a0") // small diff from A01
	DigestA06Unt = types.Digest("a06a06a06a06a06a06a06a06a06a06a0") // small diff from A01
	DigestA07Pos = types.Digest("a07a07a07a07a07a07a07a07a07a07a0") // small diff from A01
	DigestA08Pos = types.Digest("a08a08a08a08a08a08a08a08a08a08a0") // small diff from A01
	DigestA09Neg = types.Digest("a09a09a09a09a09a09a09a09a09a09a0") // large diff from A01

	// Triangle Images (of note, DigestBlank is also drawn here)
	DigestB01Pos    = types.Digest("b01b01b01b01b01b01b01b01b01b01b0")
	DigestB02Pos    = types.Digest("b02b02b02b02b02b02b02b02b02b02b0") // GREY version of B01
	DigestB03Neg    = types.Digest("b03b03b03b03b03b03b03b03b03b03b0") // big diff from B01
	DigestB04Neg    = types.Digest("b04b04b04b04b04b04b04b04b04b04b0") // truncated version of B02
	DigestB05Pos_CL = types.Digest("b05b05b05b05b05b05b05b05b05b05b0") // small diff from B02

	// Circle Images
	DigestC01Pos    = types.Digest("c01c01c01c01c01c01c01c01c01c01c0")
	DigestC02Pos    = types.Digest("c02c02c02c02c02c02c02c02c02c02c0") // GREY version of C01
	DigestC03Unt    = types.Digest("c03c03c03c03c03c03c03c03c03c03c0") // small diff from C01
	DigestC04Unt    = types.Digest("c04c04c04c04c04c04c04c04c04c04c0") // small diff from C02
	DigestC05Unt    = types.Digest("c05c05c05c05c05c05c05c05c05c05c0") // big incorrect diff from C01
	DigestC06Pos_CL = types.Digest("c06c06c06c06c06c06c06c06c06c06c0") // small diff from C01
	DigestC07Unt_CL = types.Digest("c07c07c07c07c07c07c07c07c07c07c0") // big incorrect diff from C02

	// Seven Images (it is intentional that there is only one triaged digest here).
	DigestD01Pos_CL = types.Digest("d01d01d01d01d01d01d01d01d01d01d0")

	// RoundRect Images
	DigestE01Pos_CL = types.Digest("e01e01e01e01e01e01e01e01e01e01e0")
	DigestE02Pos_CL = types.Digest("e02e02e02e02e02e02e02e02e02e02e0") // GREY version of E01
	DigestE03Unt_CL = types.Digest("e03e03e03e03e03e03e03e03e03e03e0") // big diff from E01

	// This digest is for a blank image, which has been triaged as negative on the circle test,
	// but not the others. It shows up in one other trace and one CL, where it should be seen as
	// Untriaged.
	DigestBlank = types.Digest("00000000000000000000000000000000")

	RoundCorpus   = "round"
	CornersCorpus = "corners"
	// The following corpus is added in a CL
	TextCorpus = "text"

	CircleTest   = "circle"
	SquareTest   = "square"
	TriangleTest = "triangle"
	// The following tests are added in a CL
	SevenTest     = "seven"
	RoundRectTest = "round rect"

	ColorModeKey = "color mode" // There is intentionally a space here to make sure we handle it.
	DeviceKey    = "device"
	OSKey        = "os"

	AndroidOS       = "Android"
	IOS             = "iOS"
	Windows10dot2OS = "Windows10.2"
	Windows10dot3OS = "Windows10.3"

	QuadroDevice  = "QuadroP400"
	IPadDevice    = "iPad6,3" // These happen to have a comma in it, which needs to be handled.
	IPhoneDevice  = "iPhone12,1"
	WalleyeDevice = "walleye"
	TaimenDevice  = "taimen"

	GreyColorMode = "GREY"
	RGBColorMode  = "RGB"

	UserOne        = "userOne@example.com"
	UserTwo        = "userTwo@example.com"
	UserThree      = "userThree@example.com"
	UserFour       = "userFour@example.com"
	AutoTriageUser = "fuzzy" // we use the algorithm name as the user name for auto triaging.

	GerritCRS         = "gerrit"
	GerritInternalCRS = "gerrit-internal"

	BuildBucketCIS         = "buildbucket"
	BuildBucketInternalCIS = "buildbucket-internal"

	ChangelistIDThatAttemptsToFixIOS = "CL_fix_ios"
	PatchSetIDFixesIPadButNotIPhone  = "PS_fixes_ipad_but_not_iphone"

	ChangelistIDThatAddsNewTests   = "CL_new_tests"
	PatchsetIDAddsNewCorpus        = "PS_adds_new_corpus"
	PatchsetIDAddsNewCorpusAndTest = "PS_adds_new_corpus_and_test"

	ChangelistIDThatHasLanded = "CLhaslanded"
	PatchsetIDIsLanded        = "PShaslanded"

	ChangelistIDThatIsAbandoned = "CLisabandoned"
	PatchsetIDIsAbandoned       = "PSisabandoned"

	ChangelistIDWithMultipleDatapointsPerTrace = "CLmultipledatapoints"
	PatchsetIDWithMultipleDatapointsPerTrace   = "PSmultipledatapoints"

	ChangelistIDWithDisallowTriagingTest = "CLdisallowtriaging"
	PatchsetIDWithDisallowTriagingTest   = "PSdisallowtriaging"

	Tryjob01IPhoneRGB  = "tryjob_01_iphonergb"
	Tryjob02IPad       = "tryjob_02_ipad"
	Tryjob03TaimenRGB  = "tryjob_03_taimenrgb"
	Tryjob04Windows    = "tryjob_04_windows"
	Tryjob05Windows    = "tryjob_05_windows"
	Tryjob06Walleye    = "tryjob_06_walleye"
	Tryjob07Windows    = "tryjob_07_windows"
	Tryjob08Windows    = "tryjob_08_windows"
	Tryjob09Windows    = "tryjob_09_windows"
	Tryjob10Windows    = "tryjob_10_windows"
	Tryjob11Windows    = "tryjob_11_windows"
	Tryjob12Windows    = "tryjob_12_windows"
	Tryjob13Windows    = "tryjob_13_windows"
	Tryjob14TaimenGrey = "tryjob_14_taimengrey"
)

const (
	WindowsFile1  = "gcs://skia-gold-test/dm-json-v1/2020/12/01/00/0098009800980098009800980098009800980098/waterfall/windowsfile1.json"
	WindowsFile2  = "gcs://skia-gold-test/dm-json-v1/2020/12/02/00/0099009900990099009900990099009900990099/waterfall/windowsfile2.json"
	WindowsFile3  = "gcs://skia-gold-test/dm-json-v1/2020/12/03/00/0100010001000100010001000100010001000100/waterfall/windowsfile3.json"
	WindowsFile4  = "gcs://skia-gold-test/dm-json-v1/2020/12/04/00/0101010101010101010101010101010101010101/waterfall/windowsfile4.json"
	WindowsFile5  = "gcs://skia-gold-test/dm-json-v1/2020/12/05/00/0102010201020102010201020102010201020102/waterfall/windowsfile5.json"
	WindowsFile6  = "gcs://skia-gold-test/dm-json-v1/2020/12/07/00/0106010601060106010601060106010601060106/waterfall/windowsfile6.json"
	WindowsFile7  = "gcs://skia-gold-test/dm-json-v1/2020/12/08/00/0107010701070107010701070107010701070107/waterfall/windowsfile7.json"
	WindowsFile8  = "gcs://skia-gold-test/dm-json-v1/2020/12/09/00/0108010801080108010801080108010801080108/waterfall/windowsfile8.json"
	WindowsFile9  = "gcs://skia-gold-test/dm-json-v1/2020/12/10/00/0109010901090109010901090109010901090109/waterfall/windowsfile9.json"
	WindowsFile10 = "gcs://skia-gold-test/dm-json-v1/2020/12/11/00/0110011001100110011001100110011001100110/waterfall/windowsfile10.json"

	IpadFile1  = "gcs://skia-gold-test/dm-json-v1/2020/12/01/00/0098009800980098009800980098009800980098/waterfall/ipadfile1.json"
	IpadFile2  = "gcs://skia-gold-test/dm-json-v1/2020/12/02/00/0099009900990099009900990099009900990099/waterfall/ipadfile2.json"
	IpadFile3  = "gcs://skia-gold-test/dm-json-v1/2020/12/03/00/0100010001000100010001000100010001000100/waterfall/ipadfile3.json"
	IpadFile4  = "gcs://skia-gold-test/dm-json-v1/2020/12/04/00/0101010101010101010101010101010101010101/waterfall/ipadfile4.json"
	IpadFile5  = "gcs://skia-gold-test/dm-json-v1/2020/12/05/00/0102010201020102010201020102010201020102/waterfall/ipadfile5.json"
	IpadFile6  = "gcs://skia-gold-test/dm-json-v1/2020/12/07/00/0106010601060106010601060106010601060106/waterfall/ipadfile6.json"
	IpadFile7  = "gcs://skia-gold-test/dm-json-v1/2020/12/08/00/0107010701070107010701070107010701070107/waterfall/ipadfile7.json"
	IpadFile8  = "gcs://skia-gold-test/dm-json-v1/2020/12/09/00/0108010801080108010801080108010801080108/waterfall/ipadfile8.json"
	IpadFile9  = "gcs://skia-gold-test/dm-json-v1/2020/12/10/00/0109010901090109010901090109010901090109/waterfall/ipadfile9.json"
	IpadFile10 = "gcs://skia-gold-test/dm-json-v1/2020/12/11/00/0110011001100110011001100110011001100110/waterfall/ipadfile10.json"

	IPhoneFile1a = "gcs://skia-gold-test/dm-json-v1/2020/12/01/07/0098009800980098009800980098009800980098/waterfall/iphonefile1a.json"
	IPhoneFile2b = "gcs://skia-gold-test/dm-json-v1/2020/12/02/07/0099009900990099009900990099009900990099/waterfall/iphonefile2b.json"
	IPhoneFile3a = "gcs://skia-gold-test/dm-json-v1/2020/12/03/07/0100010001000100010001000100010001000100/waterfall/iphonefile3a.json"
	IPhoneFile5a = "gcs://skia-gold-test/dm-json-v1/2020/12/05/07/0102010201020102010201020102010201020102/waterfall/iphonefile5a.json"
	IPhoneFile5b = "gcs://skia-gold-test/dm-json-v1/2020/12/05/07/0102010201020102010201020102010201020102/waterfall/iphonefile5b.json"
	IPhoneFile7a = "gcs://skia-gold-test/dm-json-v1/2020/12/08/07/0107010701070107010701070107010701070107/waterfall/iphonefile7a.json"
	IPhoneFile8b = "gcs://skia-gold-test/dm-json-v1/2020/12/09/07/0108010801080108010801080108010801080108/waterfall/iphonefile8b.json"
	IPhoneFile9a = "gcs://skia-gold-test/dm-json-v1/2020/12/10/07/0109010901090109010901090109010901090109/waterfall/iphonefile9a.json"

	WalleyeFile6  = "gcs://skia-gold-test/dm-json-v1/2020/12/07/00/0106010601060106010601060106010601060106/waterfall/walleyefile6.json"
	WalleyeFile7  = "gcs://skia-gold-test/dm-json-v1/2020/12/08/00/0107010701070107010701070107010701070107/waterfall/walleyefile7.json"
	WalleyeFile8  = "gcs://skia-gold-test/dm-json-v1/2020/12/09/00/0108010801080108010801080108010801080108/waterfall/walleyefile8.json"
	WalleyeFile9  = "gcs://skia-gold-test/dm-json-v1/2020/12/10/00/0109010901090109010901090109010901090109/waterfall/walleyefile9.json"
	WalleyeFile10 = "gcs://skia-gold-test/dm-json-v1/2020/12/11/00/0110011001100110011001100110011001100110/waterfall/walleyefile10.json"

	TaimenFile7  = "gcs://skia-gold-test/dm-json-v1/2020/12/08/00/0107010701070107010701070107010701070107/waterfall/taimenfile7.json"
	TaimenFile8  = "gcs://skia-gold-test/dm-json-v1/2020/12/09/00/0108010801080108010801080108010801080108/waterfall/taimenfile8.json"
	TaimenFile9  = "gcs://skia-gold-test/dm-json-v1/2020/12/10/00/0109010901090109010901090109010901090109/waterfall/taimenfile9.json"
	TaimenFile10 = "gcs://skia-gold-test/dm-json-v1/2020/12/11/00/0110011001100110011001100110011001100110/waterfall/taimenfile10.json"

	Tryjob01FileIPhoneRGB  = "gcs://skia-gold-test/trybot/dm-json-v1/2020/12/10/04/PS_fixes_ipad_but_not_iphone/iphonergb.json"
	Tryjob02FileIPad       = "gcs://skia-gold-test/trybot/dm-json-v1/2020/12/10/03/PS_fixes_ipad_but_not_iphone/ipad.json"
	Tryjob03FileTaimenRGB  = "gcs://skia-gold-test/trybot/dm-json-v1/2020/12/10/03/PS_fixes_ipad_but_not_iphone/taimen.json"
	Tryjob04FileWindows    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/12/12/08/PS_adds_new_corpus/windows.json"
	Tryjob05FileWindows    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/12/10/09/PS_adds_new_corpus_and_test/windows.json"
	Tryjob06FileWalleye    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/12/10/09/PS_adds_new_corpus_and_test/walleye.json"
	Tryjob07FileWindows    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/05/05/05/PShaslanded/windows.json"
	Tryjob08FileWindows    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/06/06/06/PSisabandoned/windows.json"
	Tryjob09FileWindows    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/07/07/07/PSmultipledatapoints/windows.json"
	Tryjob10FileWindows    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/07/07/08/PSmultipledatapoints/windows.json"
	Tryjob11FileWindows    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/07/07/09/PSmultipledatapoints/windows.json"
	Tryjob12FileWindows    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/07/07/10/PSmultipledatapoints/windows.json"
	Tryjob13FileWindows    = "gcs://skia-gold-test/trybot/dm-json-v1/2020/07/07/11/PSmultipledatapoints/windows.json"
	Tryjob14FileTaimenGrey = "gcs://skia-gold-test/trybot/dm-json-v1/2020/08/08/08/PSdisallowtriaging/taimengrey.json"
)

const (
	// Here are the git hashes in commit order:
	//"d6ccb84da1169020d632bae7a5db48e6aa4c6767",
	//"248ad6d0594212b32036b3ca687ab4af9270ca43",
	//"77e1d3255fde0bebc80f596f6a1c4ad79c8b8bbe",
	//"c43edfb29110b09c89eac7359659151e2a9fc047",
	//"6abbeb6272480820e7fc00392b771a71892563a5",
	//"0103010301030103010301030103010301030103",
	//"0104010401040104010401040104010401040104",
	//"0105010501050105010501050105010501050105",
	//"84c2c3c8a79279939973884e01c7a183c00f0077",
	//"200bbd369fac004c716d61fd8f45f8ba305ab004",
	//"7bdc929af0c52a997bb6d5ccadbe86ddc532a25a",
	//"0b1a16e767f3accd50da5acfbf953ce0c66a0183",
	//"f4412901bfb130a8774c0c719450d1450845f471",
	//"acb4c0deaa115916044699809a363d539cf725da",

	OldestCommitID = schema.CommitID("0000000098")

	// The windows driver was updated on the 4th commit with data.
	WindowsDriverUpdateCommitID = schema.CommitID("0000000101")

	// The triangle tests were fixed for IOS on the 8th commit with data. This had the side effect
	// of breaking the circle tests.
	IOSFixTriangleTestsBreakCircleTestsCommitID = schema.CommitID("0000000108")

	// MostRecentCommitID is the most recent commit id with data
	MostRecentCommitID = schema.CommitID("0000000110")
)

// These are the computed trace ids for the tests in the example data. Their values are validated
// via a test.
const (
	CircleGroupingIDHex    = "d612d801ac91001dd4b0024883aaf742"
	RoundRectGroupingIDHex = "004478668604fac88f4bb811884d1b40"
	SquareGroupingIDHex    = "0f2ffd3aef866dc6155bcbc5697b0604"
	TextSevenGroupingIDHex = "8a58b09f0dbc06aeba43cf53ec66e830"
	TriangleGroupingIDHex  = "aa8d3c14238a4f717b9a99f7fe3735a7"
)

var (
	CircleGroupingID    = mustDecode(CircleGroupingIDHex)
	RoundRectGroupingID = mustDecode(RoundRectGroupingIDHex)
	SquareGroupingID    = mustDecode(SquareGroupingIDHex)
	TextSevenGroupingID = mustDecode(TextSevenGroupingIDHex)
	TriangleGroupingID  = mustDecode(TriangleGroupingIDHex)
)

var (
	Tryjob01LastIngested = time.Date(2020, time.December, 10, 4, 5, 6, 0, time.UTC)
	Tryjob02LastIngested = time.Date(2020, time.December, 10, 3, 2, 1, 0, time.UTC)
	Tryjob03LastIngested = time.Date(2020, time.December, 10, 3, 44, 44, 0, time.UTC)
	Tryjob04LastIngested = time.Date(2020, time.December, 12, 8, 9, 10, 0, time.UTC)
)

func mustDecode(hexString string) schema.GroupingID {
	b, err := hex.DecodeString(hexString)
	if err != nil {
		panic(err)
	}
	return b
}
