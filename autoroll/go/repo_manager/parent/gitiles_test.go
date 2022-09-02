package parent

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/gerrit/mocks"
)

func TestHandleExternalChangeId(t *testing.T) {
	ctx := context.Background()

	// Test values.
	origChanges := map[string]string{
		"DEPS": "abc xyz",
	}
	clNum := int64(12345)
	clChanges := map[string]string{
		"file1":      "xyz",
		"dir1/file2": "abc",
		"file3":      "",
	}

	// Setup mock for Gerrit.
	g := &mocks.GerritInterface{}
	g.On("GetFilesToContent", ctx, clNum, "current").Return(clChanges, nil)

	// Check for expected values.
	err := handleExternalChangeId(ctx, origChanges, strconv.FormatInt(clNum, 10), g)
	assert.NoError(t, err)
	assert.Len(t, origChanges, 4)
	assert.Equal(t, "abc xyz", origChanges["DEPS"])
	assert.Equal(t, "abc", origChanges["dir1/file2"])
}

func TestHandleExternalChangeId_InvalidExternalChangeId(t *testing.T) {

	err := handleExternalChangeId(context.Background(), map[string]string{"DEPS": "xyz"}, "invalid-change-num", nil)
	fmt.Println(err)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "invalid syntax"))
}

func TestHandleExternalChange_MergeConflict(t *testing.T) {
	ctx := context.Background()

	// Test values.
	origChanges := map[string]string{
		"DEPS": "abc xyz",
	}
	clNum := int64(12345)
	clChanges := map[string]string{
		"dir1/file1": "abc",
		"DEPS":       "123",
	}

	// Setup mock for Gerrit.
	g := &mocks.GerritInterface{}
	g.On("GetFilesToContent", ctx, clNum, "current").Return(clChanges, nil)

	// Check for error due to merge conflict because DEPS is modified in both
	// places.
	err := handleExternalChangeId(ctx, origChanges, strconv.FormatInt(clNum, 10), g)
	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "DEPS already modified by the roll"))
}
