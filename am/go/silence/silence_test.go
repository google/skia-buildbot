package silence

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils"
)

func TestStore(t *testing.T) {
	testutils.LargeTest(t)

	cleanup := testutil.InitDatastore(t, ds.SILENCE_AM)
	defer cleanup()

	st := NewStore(ds.DS)
	s := &Silence{
		User: "fred@example.org",
		ParamSet: paramtools.ParamSet{
			"alertname": []string{"BotQuarantined"},
			"bot":       []string{"skia-rpi-104", "skia-rpi-114"},
		},
		Created:  time.Now().Unix(),
		Duration: time.Minute,
	}

	var err error
	s, err = st.Create(s)
	assert.NoError(t, err)
	assert.True(t, s.Active)
	assert.NotEqual(t, "", s.Key)

	all, err := st.GetAll()
	assert.Len(t, all, 1)
}
