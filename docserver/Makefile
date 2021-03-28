build:
	go install ./go/...

build-local-image: build
	SKIP_UPLOAD=1 ./images/release

test:
	go test ./go/...

release:
	./images/release

build-using-docker-image:
	docker run --entrypoint=/build.sh -ti -p 1313:1313 -v ~/projects/skia/site2:/input docsy:latest

serve-using-docker-image:
	docker run --entrypoint=/serve.sh  -ti -p 1313:1313 -v ~/projects/skia/site2:/input -v /tmp/output/:/output  docsy:latest