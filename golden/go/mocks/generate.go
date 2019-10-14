package mocks

//go:generate mockery -name BaselineFetcher -dir ../baseline/ -output .
//go:generate mockery -name ClosestDiffFinder -dir ../digesttools -output .
//go:generate mockery -name ComplexTile -dir ../types -output .
//go:generate mockery -name DigestCounter -dir ../digest_counter -output .
//go:generate mockery -name ExpectationsStore -dir ../expstorage -output .
//go:generate mockery -name GCSClient -dir ../storage -output .
//go:generate mockery -name TileSource -dir ../tilesource -output .
//go:generate mockery -name TraceStore -dir ../tracestore -output .
