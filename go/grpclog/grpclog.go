package grpclog

import (
	"github.com/skia-dev/glog"
	grl "google.golang.org/grpc/grpclog"
)

// logger implements grpclog.Logger using glog.
type logger struct{}

func (g *logger) Fatal(args ...interface{}) {
	glog.Fatal(args...)
}
func (g *logger) Fatalf(format string, args ...interface{}) {
	glog.Fatalf(format, args...)
}
func (g *logger) Fatalln(args ...interface{}) {
	glog.Fatalln(args...)
}
func (g *logger) Print(args ...interface{}) {
	glog.Info(args...)
}
func (g *logger) Printf(format string, args ...interface{}) {
	glog.Infof(format, args...)
}
func (g *logger) Println(args ...interface{}) {
	glog.Infoln(args...)
}

// Init sets up grpc logging using glog.
func Init() {
	grl.SetLogger(&logger{})
}
