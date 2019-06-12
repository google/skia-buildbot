KGO := CGO_ENABLED=0 GOOS=linux go build

default:
	go install -v ./go/traceserver
	go install -v ./go/tracetool
	go install -v ./go/importtile
	go install -v ./go/difftile

.PHONY: k8s-binaries
k8s-binaries:
	mkdir -p ./build
	rm -f ./build/*
	$(KGO) -o build/traceserver_k8s  -a ./go/traceserver/main.go

.PHONY: k8s-release-traceserver
k8s-release-traceserver: k8s-binaries
	./k8s_push_traceserver
