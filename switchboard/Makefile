build:
	go install ./go/...

release: build
	./images/switch-pod/release

push: release
	pushk switch-pod