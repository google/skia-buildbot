default:
	go install -v ./go/certpoller

loadtest:
	go install -v ./go/loadtest

release: default
	./build_release "$(MESSAGE)"