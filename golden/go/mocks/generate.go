package mocks

//go:generate mockery --name ComplexTile --dir ../tiling --output .
//go:generate mockery --name DigestCounter --dir ../digest_counter --output .
//go:generate mockery --name GCSClient --dir ../storage --output .
//go:generate mockery --name TileSource --dir ../tilesource --output .
//go:generate mockery --name TraceStore --dir ../tracestore --output .
