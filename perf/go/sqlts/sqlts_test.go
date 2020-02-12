package sqlts

import (
	"fmt"
	"io/ioutil"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestNewSQLite(t *testing.T) {
	unittest.SmallTest(t)
	tmpfile, err := ioutil.TempFile("", "sqlts")
	assert.NoError(t, err)
	err = tmpfile.Close()
	assert.NoError(t, err)

	fmt.Println(tmpfile.Name())
	//defer os.Remove(tmpfile.Name()) // clean up
	_, err = NewSQLite(tmpfile.Name())
	assert.NoError(t, err)
}
