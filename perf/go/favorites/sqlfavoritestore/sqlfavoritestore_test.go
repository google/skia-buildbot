package sqlfavoritestore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/favorites"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func setUp(t *testing.T) (favorites.Store, pool.Pool) {
	db := sqltest.NewCockroachDBForTests(t, "favoritestore")
	store := New(db)

	return store, db
}

func TestGet_FavoriteWithId(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	f1 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav1",
		Url:         "url/fav1",
		Description: "Desc for fav1",
	}

	f2 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav2",
		Url:         "url/fav2",
		Description: "Desc for fav2",
	}

	f3 := &favorites.SaveRequest{
		UserId:      "b@c.com",
		Name:        "fav3",
		Url:         "url/fav3",
		Description: "Desc for fav3",
	}

	err := store.Create(ctx, f1)
	require.NoError(t, err)
	err = store.Create(ctx, f2)
	require.NoError(t, err)
	err = store.Create(ctx, f3)
	require.NoError(t, err)

	favs, err := store.List(ctx, "a@b.com")
	require.NoError(t, err)

	getId := favs[0].ID
	favFromDb, err := store.Get(ctx, getId)
	require.NoError(t, err)

	require.Equal(t, getId, favFromDb.ID)
}

func TestGet_NonExistentFavorite_ReturnsError(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	f1 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav1",
		Url:         "url/fav1",
		Description: "Desc for fav1",
	}

	err := store.Create(ctx, f1)
	require.NoError(t, err)

	_, err = store.Get(ctx, "10")
	require.Error(t, err)
}

func TestCreate(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	req1 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "Fav1",
		Description: "Desc for Fav1",
		Url:         "a_b.com",
	}

	req2 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "Fav2",
		Description: "Desc for Fav2",
		Url:         "a_b_1.com",
	}

	req3 := &favorites.SaveRequest{
		UserId:      "b@c.com",
		Name:        "Fav3",
		Description: "Desc for Fav3",
		Url:         "b_c.com",
	}

	err := store.Create(ctx, req1)
	require.NoError(t, err)

	err = store.Create(ctx, req2)
	require.NoError(t, err)

	err = store.Create(ctx, req3)
	require.NoError(t, err)

	favs, err := store.List(ctx, "a@b.com")
	require.NoError(t, err)
	require.Len(t, favs, 2)
}

func TestUpdate_ExistingFavorite(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	f1 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav1",
		Url:         "url/fav1",
		Description: "Desc for fav1",
	}

	f2 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav2",
		Url:         "url/fav2",
		Description: "Desc for fav2",
	}

	f3 := &favorites.SaveRequest{
		UserId:      "b@c.com",
		Name:        "fav3",
		Url:         "url/fav3",
		Description: "Desc for fav3",
	}

	err := store.Create(ctx, f1)
	require.NoError(t, err)
	err = store.Create(ctx, f2)
	require.NoError(t, err)
	err = store.Create(ctx, f3)
	require.NoError(t, err)

	favs, err := store.List(ctx, "a@b.com")
	require.NoError(t, err)

	favId := favs[0].ID

	f := &favorites.SaveRequest{
		Name:        "fav1updated",
		Url:         "url/fav1/updated",
		Description: "Desc for fav1 updated",
	}
	err = store.Update(ctx, f, favId)
	require.NoError(t, err)

	updatedFav, err := store.Get(ctx, favId)
	require.NoError(t, err)

	require.Equal(t, "fav1updated", updatedFav.Name)
	require.Equal(t, "Desc for fav1 updated", updatedFav.Description)
	require.Equal(t, "url/fav1/updated", updatedFav.Url)
}

func TestUpdate_NonExistingFavorite(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	f1 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav1",
		Url:         "url/fav1",
		Description: "Desc for fav1",
	}

	f2 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav2",
		Url:         "url/fav2",
		Description: "Desc for fav2",
	}

	f3 := &favorites.SaveRequest{
		UserId:      "b@c.com",
		Name:        "fav3",
		Url:         "url/fav3",
		Description: "Desc for fav3",
	}

	err := store.Create(ctx, f1)
	require.NoError(t, err)
	err = store.Create(ctx, f2)
	require.NoError(t, err)
	err = store.Create(ctx, f3)
	require.NoError(t, err)

	nonExistentFavId := "10"

	req := &favorites.SaveRequest{
		Name:        "fav1updated",
		Url:         "url/fav1/updated",
		Description: "Desc for fav1 updated",
	}
	err = store.Update(ctx, req, nonExistentFavId)
	require.Error(t, err)

	favs, err := store.List(ctx, "a@b.com")
	require.NoError(t, err)

	require.Len(t, favs, 2)
	require.Contains(t, []string{favs[0].Name, favs[1].Name}, "fav1")
	require.Contains(t, []string{favs[0].Name, favs[1].Name}, "fav2")
}

func TestDelete_FavoriteWithId(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	f1 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav1",
		Url:         "url/fav1",
		Description: "Desc for fav1",
	}

	f2 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav2",
		Url:         "url/fav2",
		Description: "Desc for fav2",
	}

	err := store.Create(ctx, f1)
	require.NoError(t, err)

	err = store.Create(ctx, f2)
	require.NoError(t, err)

	favsInDb, err := store.List(ctx, "a@b.com")
	require.NoError(t, err)
	require.Equal(t, 2, len(favsInDb))

	deleteId := favsInDb[0].ID
	err = store.Delete(ctx, "a@b.com", deleteId)
	require.NoError(t, err)

	favsInDb, err = store.List(ctx, "a@b.com")
	require.Equal(t, 1, len(favsInDb))
	require.NotEqual(t, deleteId, favsInDb[0].ID)
}

func TestList_ForUserId(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	f1 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav1",
		Url:         "url/fav1",
		Description: "Desc for fav1",
	}

	f2 := &favorites.SaveRequest{
		UserId:      "a@b.com",
		Name:        "fav2",
		Url:         "url/fav2",
		Description: "Desc for fav2",
	}

	f3 := &favorites.SaveRequest{
		UserId:      "b@c.com",
		Name:        "fav3",
		Url:         "url/fav3",
		Description: "Desc for fav3",
	}

	err := store.Create(ctx, f1)
	require.NoError(t, err)

	err = store.Create(ctx, f2)
	require.NoError(t, err)

	err = store.Create(ctx, f3)
	require.NoError(t, err)

	favFromDb, err := store.List(ctx, "a@b.com")
	require.NoError(t, err)
	require.Len(t, favFromDb, 2)
	require.Contains(t, []string{favFromDb[0].Name, favFromDb[1].Name}, "fav1")
	require.Contains(t, []string{favFromDb[0].Name, favFromDb[1].Name}, "fav2")
}

func TestList_ForUserId_EmptyList(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	favFromDb, err := store.List(ctx, "a@b.com")
	require.NoError(t, err)
	require.Len(t, favFromDb, 0)
}

func TestLiveness_GivenNoData_ReturnsNoError(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	err := store.Liveness(ctx)
	require.NoError(t, err)
}
