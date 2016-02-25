default:
	go install -v ./go/prober

push: default
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli proberd skia-monitoring

release:
	build_release
