package mocks

//go:generate mockery -name Baseliner -dir ../baseline/ -output .
//go:generate mockery -name ClosestDiffFinder -dir ../digesttools -output .
//go:generate mockery -name ComplexTile -dir ../types -output .
//go:generate mockery -name DiffStore -dir ../diff -output .
//go:generate mockery -name DiffWarmer -dir ../warmer -output .
//go:generate mockery -name DigestCounter -dir ../digest_counter -output .
//go:generate mockery -name ExpectationsStore -dir ../expstorage -output .
//go:generate mockery -name GCSClient -dir ../storage -output .
//go:generate mockery -name TileInfo -dir ../baseline -output .
//go:generate mockery -name TileSource -dir ../storage -output .
//go:generate mockery -name TryjobStore -dir ../tryjobstore -output .
