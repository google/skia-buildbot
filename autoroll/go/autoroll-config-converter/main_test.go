package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSortPrivacySandboxVersionSlice(t *testing.T) {
	// Check inequality.
	checkLess := func(less, more *PrivacySandboxVersion) {
		s := PrivacySandboxVersionSlice([]*PrivacySandboxVersion{less, more})
		require.True(t, s.Less(0, 1))
		require.False(t, s.Less(1, 0))
		require.False(t, s.Less(0, 0))
		require.False(t, s.Less(1, 1))
	}
	checkLess(&PrivacySandboxVersion{
		BranchName: "less",
	}, &PrivacySandboxVersion{
		BranchName: "more",
	})
	checkLess(&PrivacySandboxVersion{
		Ref: "less",
	}, &PrivacySandboxVersion{
		Ref: "more",
	})
	checkLess(&PrivacySandboxVersion{
		Bucket: "less",
	}, &PrivacySandboxVersion{
		Bucket: "more",
	})
	checkLess(&PrivacySandboxVersion{
		PylFile: "less",
	}, &PrivacySandboxVersion{
		PylFile: "more",
	})
	checkLess(&PrivacySandboxVersion{
		PylTargetPath: "less",
	}, &PrivacySandboxVersion{
		PylTargetPath: "more",
	})
	checkLess(&PrivacySandboxVersion{
		CipdPackage: "less",
	}, &PrivacySandboxVersion{
		CipdPackage: "more",
	})
	checkLess(&PrivacySandboxVersion{
		CipdTag: "less",
	}, &PrivacySandboxVersion{
		CipdTag: "more",
	})

	// Check equality.
	s := PrivacySandboxVersionSlice([]*PrivacySandboxVersion{{}, {}})
	require.False(t, s.Less(0, 1))
	require.False(t, s.Less(1, 0))
	require.False(t, s.Less(0, 0))
	require.False(t, s.Less(1, 1))
}
