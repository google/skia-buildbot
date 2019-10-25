package grpclog

import (
	"go.skia.org/infra/go/sklog"
	grl "google.golang.org/grpc/grpclog"
)

// logger implements grpclog.Logger using sklog.
type logger struct{}

func (g *logger) Fatal(args ...interface{}) {
	sklog.Fatal(args...)
}
func (g *logger) Fatalf(format string, args ...interface{}) {
	sklog.Fatalf(format, args...)
}
func (g *logger) Fatalln(args ...interface{}) {
	sklog.Fatal(args...)
}
func (g *logger) Print(args ...interface{}) {
	sklog.Info(args...)
}
func (g *logger) Printf(format string, args ...interface{}) {
	sklog.Infof(format, args...)
}
func (g *logger) Println(args ...interface{}) {
	sklog.Info(args...)
}

// Init sets up grpc logging using sklog.
func Init() {
	grl.SetLogger(&logger{})
}
