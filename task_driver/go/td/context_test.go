package td

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetEnv(t *testing.T) {
	tr := StartTestRun(t)
	defer tr.Cleanup()

	// Root-level step.
	ctx := tr.Root()

	tests := []struct {
		name  string
		ctx   context.Context
		empty bool
		want  []string
	}{
		{
			name:  "empty",
			ctx:   context.Background(),
			empty: true,
			want:  []string{},
		},
		{
			name:  "simple",
			ctx:   ctx,
			empty: false,
			want:  []string{},
		},
		{
			name:  "one var",
			ctx:   WithEnv(ctx, []string{"FOO=bar"}),
			empty: false,
			want:  []string{"FOO=bar"},
		},
		{
			name:  "two vars",
			ctx:   WithEnv(WithEnv(ctx, []string{"FOO=bar"}), []string{"BAR=baz"}),
			empty: false,
			want:  []string{"FOO=bar", "BAR=baz"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GetEnv(tt.ctx)
			if tt.empty {
				assert.Len(t, got, 0)
			} else {
				for _, want := range tt.want {
					assert.Contains(t, got, want)
				}
			}
		})
	}
}
