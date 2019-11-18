// docset keeps track of checkouts of a repository of Markdown documents.
package docset

import (
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
)

func Test_readTitle(t *testing.T) {
	unittest.SmallTest(t)
	tests := []struct {
		name     string
		filename string
		def      string
		want     string
	}{
		{
			name:     "Actual file",
			filename: "testdata/somefile.md",
			def:      "",
			want:     "This is a title",
		},
		{
			name:     "Missinge file",
			filename: "testdata/not-an-actual-file.md",
			def:      "The default title",
			want:     "The default title",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := readTitle(tt.filename, tt.def); got != tt.want {
				t.Errorf("readTitle() = %v, want %v", got, tt.want)
			}
		})
	}
}
