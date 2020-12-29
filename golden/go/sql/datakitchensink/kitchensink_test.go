package datakitchensink_test

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

// If this test fails, it may mean the data needs to be regenerated via `make generate` to match
// any changes in the schema.
func TestImportTSVData_DataIsValidAndMatchesSchema(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)

	// Find the tsv folder that is in the same directory as this file.
	_, thisFile, _, ok := runtime.Caller(0)
	require.True(t, ok)
	localTSV := filepath.Join(filepath.Dir(thisFile), "tsv")

	importTSVIntoDB(ctx, t, localTSV, db)

	// Spot check the data.
	row := db.QueryRow(ctx, "SELECT count(*) from TraceValues")
	count := 0
	assert.NoError(t, row.Scan(&count))
	assert.Equal(t, 2, count)
}

func importTSVIntoDB(ctx context.Context, t *testing.T, pathToTSVs string, db *pgxpool.Pool) {
	// To import local TSV files into cockroachdb, we need to spin up an HTTP file server that
	// serves them. This only need be up until after the import finishes.
	fs := http.FileServer(http.Dir(pathToTSVs))
	server := &http.Server{Addr: "localhost:10345", Handler: fs}
	go func() {
		err := server.ListenAndServe()
		if err != http.ErrServerClosed {
			require.NoError(t, err)
		}
	}()
	defer util.Close(server)
	// We read in the TSV files (ReadDir sorts these alphabetically).
	xfi, err := ioutil.ReadDir(pathToTSVs)
	require.NoError(t, err)
	var files []interface{}
	for _, info := range xfi {
		n := info.Name()
		if strings.HasSuffix(n, ".tsv") {
			files = append(files, "http://localhost:10345/"+n)
		}
	}
	// After reading in the TSV files, we apply them as arguments to the template; we assume that
	// the Tables are listed in alphabetical order as well.
	importStatement := fmt.Sprintf(schema.ImportTSVTemplate, files...)

	_, err = db.Exec(ctx, importStatement)
	require.NoError(t, err, "importing with %s", importStatement)
}
