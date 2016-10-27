default:
	go install -v ./go/...

push: default
	./build_release "`git log -n1 --format=%s`"
	go install -v ../push/go/pushcli
	pushcli cq-watcherd skia-cq-watcher
