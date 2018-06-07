package script

import "go.skia.org/infra/go/sklog"

func FatalIfErr(err error, formatStr string) {
	if err != nil {
		sklog.Fatalf(formatStr, err)
	}
}

func Fatalf(formatStr string, args ...interface{}) {
	sklog.Fatalf(formatStr, args...)
}

func FatalAny(errs ...interface{}) {
	for _, potentialErr := range errs {
		if err, ok := potentialErr.(error); ok && err != nil {
			sklog.Fatalf("Error: %s", err)
		}
	}
}
