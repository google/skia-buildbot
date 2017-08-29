default:
	go install -v ./go/certpoller

loadtest:
	go install -v ./go/loadtest

release: default
	./build_release "`git log -n1 --format=%s`"