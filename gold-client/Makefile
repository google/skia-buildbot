KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: default
default: allgo

include ../go/skiaversion/skiaversion.mk

.PHONY: test
test:
	go test ./go/... -v

.PHONY: allgo
allgo: skiaversion
	go install -v ./cmd/...

.PHONY: release-goldctl-gcs
release-goldctl-gcs: build-static-goldctl
	./release_goldctl

.PHONY: build-static-goldctl
build-static-goldctl:
	mkdir -p ./build
	rm -f ./build/goldctl
	$(KGO) -o build/goldctl -a ./cmd/goldctl/...

.PHONY: mocks
mocks:
	go get github.com/vektra/mockery/.../
	# We must make this one in the same package to avoid circular dependency
	mockery -name AuthOpt -dir go/goldclient/ -inpkg
	mockery -name HTTPClient -dir go/goldclient/ -output ./go/mocks/
	mockery -name GoldClient -dir go/goldclient/ -output ./go/mocks/
	mockery -name GoldUploader -dir go/goldclient/ -output ./go/mocks/