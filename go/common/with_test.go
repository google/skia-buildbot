package common

import (
	"flag"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFlagSetOpt_UsingFlagSetOptChangesFlagSet_Success(t *testing.T) {

	myFlagSet := flag.NewFlagSet("my-flagset-name", flag.ContinueOnError)
	err := InitWith("my-app-name", FlagSetOpt(myFlagSet))

	// Expected to fail since this will parse the os.Args of this unit test, and
	// we haven't specified any flags.
	require.Error(t, err)
	require.Equal(t, myFlagSet, FlagSet)
	require.True(t, FlagSet.Parsed())
}
