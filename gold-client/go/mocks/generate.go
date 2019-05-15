package mocks

//go:generate mockery -name HTTPClient -dir ../goldclient -output .
//go:generate mockery -name GoldClient -dir ../goldclient -output .
//go:generate mockery -name GoldUploader -dir ../goldclient -output .

// We must make this one in the same package to avoid circular dependency
//go:generate mockery -name AuthOpt -dir ../goldclient/ -inpkg
