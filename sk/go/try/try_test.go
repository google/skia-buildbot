package try

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
)

// mockTryJobReader is a mock implementation of tryJobReader used for testing.
type mockTryJobReader struct {
	jobs map[string][]string
}

// getTryJobs implements tryJobReader.
func (r *mockTryJobReader) getTryJobs(ctx context.Context) (map[string][]string, error) {
	return r.jobs, nil
}

func TestTry(t *testing.T) {
	unittest.SmallTest(t)

	mockCmd := exec.CommandCollector{}
	ctx := exec.NewContext(context.Background(), mockCmd.Run)
	bucket := "skia/skia.primary"
	tryjobs = &mockTryJobReader{
		jobs: map[string][]string{
			bucket: {
				"my-job",
				"another-job",
			},
		},
	}
	tryCmdPrefix := []string{"cl", "try", "-B", bucket}

	check := func(jobs []string, noPrompt bool, input string, expectTriggered []string) {
		mockCmd.ClearCommands()
		stdin = strings.NewReader(input)
		require.NoError(t, try(ctx, jobs, noPrompt, ""))
		triggeredJobs := []string{}
		for _, cmd := range mockCmd.Commands() {
			if len(cmd.Args) > len(tryCmdPrefix) && util.SSliceEqual(cmd.Args[:len(tryCmdPrefix)], tryCmdPrefix) {
				for i := len(tryCmdPrefix); i < len(cmd.Args); i++ {
					arg := cmd.Args[i]
					if arg != "-b" {
						triggeredJobs = append(triggeredJobs, arg)
					}
				}
			}
		}
		require.Equal(t, expectTriggered, triggeredJobs)
	}
	check([]string{"my-job"}, true, "", []string{"my-job"})
	check([]string{"my-job"}, false, "y\n", []string{"my-job"})
	check([]string{".*-job"}, true, "", []string{"another-job", "my-job"})
	check([]string{"my-job"}, false, "n\n", []string{})
	check([]string{".*-job"}, false, "i\ny\ny\n", []string{"another-job", "my-job"})
	check([]string{".*-job"}, false, "i\nn\ny\n", []string{"my-job"})
}
