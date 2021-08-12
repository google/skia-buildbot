build:
	go install ./go/...

release: build
	./images/switch-pod/release

push: release
	pushk switch-pod

test:
	go test ./go/...

run-local-instance: build
	switch-pod-monitor