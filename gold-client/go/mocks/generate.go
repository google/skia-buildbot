package mocks

//go:generate mockery --name ImageDownloader --dir ../imagedownloader --output .
//go:generate mockery --name GCSUploader --dir ../gcsuploader --output .
//go:generate mockery --name HTTPClient --dir ../httpclient --output .
//go:generate mockery --name Matcher --dir ../imgmatching --output .
