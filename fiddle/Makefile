build:
	go install -v ./go/fiddle_build
	go install -v ./go/fiddle_run
	go install -v ./go/webtry_extract
	go install -v ./go/fiddle_run_all
	go install -v ./go/fiddle
	c++ fiddle_secwrap.cpp -o fiddle_secwrap

testgo:
	go test -v ./...

push: build
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli fiddle skia-fiddle

