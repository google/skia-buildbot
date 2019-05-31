package fs_expstore

import (
	"context"
	"sync"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils/unittest"
	data "go.skia.org/infra/golden/go/testutils/data_three_devices"
	"go.skia.org/infra/golden/go/types"
)

func TestGetExpectations(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t, "get-test-1")

	f := New(c, MasterBranch, ReadWrite)

	// Brand new instance should have no expectations
	e, err := f.Get()
	assert.NoError(t, err)
	assert.Equal(t, types.Expectations{}, e)

	ctx := context.Background()
	err = f.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaUntriaged1Digest: types.POSITIVE,
			data.AlphaGood1Digest:      types.POSITIVE,
		},
	}, userOne)
	assert.NoError(t, err)
	err = f.AddChange(ctx, types.Expectations{
		data.AlphaTest: {
			data.AlphaBad1Digest:       types.NEGATIVE,
			data.AlphaUntriaged1Digest: types.UNTRIAGED, // overwrites previous
		},
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}, userTwo)
	assert.NoError(t, err)

	e, err = f.Get()
	assert.NoError(t, err)
	assert.Equal(t, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest:      types.POSITIVE,
			data.AlphaBad1Digest:       types.NEGATIVE,
			data.AlphaUntriaged1Digest: types.UNTRIAGED,
		},
		data.BetaTest: {
			data.BetaGood1Digest: types.POSITIVE,
		},
	}, e)
}

func TestGetExpectationsRace(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresFirestoreEmulator(t)

	c := getTestFirestoreInstance(t, "get-test-2")

	f := New(c, MasterBranch, ReadWrite)

	type entry struct {
		Grouping types.TestName
		Digest   types.Digest
		Label    types.Label
	}

	entries := []entry{
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaUntriaged1Digest,
			Label:    types.UNTRIAGED,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaBad1Digest,
			Label:    types.NEGATIVE,
		},
		{
			Grouping: data.AlphaTest,
			Digest:   data.AlphaGood1Digest,
			Label:    types.POSITIVE,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaGood1Digest,
			Label:    types.POSITIVE,
		},
		{
			Grouping: data.BetaTest,
			Digest:   data.BetaUntriaged1Digest,
			Label:    types.UNTRIAGED,
		},
	}

	ctx := context.Background()
	wg := sync.WaitGroup{}

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			e := entries[i%len(entries)]
			err := f.AddChange(ctx, types.Expectations{
				e.Grouping: {
					e.Digest: e.Label,
				},
			}, userOne)
			assert.NoError(t, err)
		}(i)
	}

	wg.Wait()

	e, err := f.Get()
	assert.NoError(t, err)
	assert.Equal(t, types.Expectations{
		data.AlphaTest: {
			data.AlphaGood1Digest:      types.POSITIVE,
			data.AlphaBad1Digest:       types.NEGATIVE,
			data.AlphaUntriaged1Digest: types.UNTRIAGED,
		},
		data.BetaTest: {
			data.BetaGood1Digest:      types.POSITIVE,
			data.BetaUntriaged1Digest: types.UNTRIAGED,
		},
	}, e)
}

func getTestFirestoreInstance(t *testing.T, collectionId string) *firestore.Client {
	c, err := firestore.NewClient(context.Background(), "should-use-emulator", "gold-test", ExpectationStoreCollection+collectionId, nil)
	assert.NoError(t, err)
	return c
}

const (
	userOne = "userOne@example.com"
	userTwo = "userTwo@example.com"
)
