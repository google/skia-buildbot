package goldclient

import (
	"testing"
	"time"

	"go.skia.org/infra/golden/go/jsonio"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/now"
)

func TestGetResultFilePath_Success(t *testing.T) {

	arbitraryTime := time.Date(2022, 1, 2, 3, 4, 5, 67, time.UTC)
	ctx := now.TimeTravelingContext(arbitraryTime)

	test := func(name string, config jsonio.GoldResults, expectedPath string) {
		t.Run(name, func(t *testing.T) {
			rs := resultState{SharedConfig: config}
			path := rs.getResultFilePath(ctx)
			assert.Equal(t, expectedPath, path)
		})
	}

	test("basic_ci", jsonio.GoldResults{
		GitHash:       "abcdef1234567890abcdef1234567890",
		CommitID:      "",
		ChangelistID:  "",
		PatchsetID:    "",
		PatchsetOrder: 0,
		TryJobID:      "",
	}, "/dm-json-v1/2022/01/02/03/abcdef1234567890abcdef1234567890/waterfall/dm-1641092645000000067.json")
	test("gerrit_cl", jsonio.GoldResults{
		GitHash:       "abcdef1234567890abcdef1234567890", // ignored
		CommitID:      "",
		ChangelistID:  "776655", // e.g. a Gerrit ID
		PatchsetID:    "",       // Typical gerrit usage does not include the ID
		PatchsetOrder: 8,
		TryJobID:      "654321",
	}, "/trybot/dm-json-v1/2022/01/02/03/776655__8/654321/dm-1641092645000000067.json")
	test("github_pr", jsonio.GoldResults{
		GitHash:       "abcdef1234567890abcdef1234567890", // ignored
		CommitID:      "",
		ChangelistID:  "680", // e.g. GitHub PR
		PatchsetID:    "00112233445566778899aabbccddeeff",
		PatchsetOrder: 0, // Typical GitHub usage does not include the order
		TryJobID:      "765432",
	}, "/trybot/dm-json-v1/2022/01/02/03/680_00112233445566778899aabbccddeeff_0/765432/dm-1641092645000000067.json")

}
