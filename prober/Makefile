default:
	go install -v ./go/prober ./go/build_probers_json5
	build_probers_json5 --dest=allprobers.json5

push: default
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli proberd skia-monitoring

release:
	build_release
