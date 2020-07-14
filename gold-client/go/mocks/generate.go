package mocks

//go:generate mockery -name ImageDownloader -dir ../goldclient -output .
//go:generate mockery -name GCSUploader -dir ../goldclient -output .
//go:generate mockery -name GoldClient -dir ../goldclient -output .
//go:generate mockery -name HTTPClient -dir ../goldclient -output .
//go:generate mockery -name Matcher -dir ../imgmatching -output .
