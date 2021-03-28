build:
	go install ./go/...

release:
	./images/release

serve:
	docker run --entrypoint=/build.sh -ti -p 1313:1313 -v ~/projects/skia/site2:/input docsy:latest

build:
	docker run --entrypoint=/serve.sh  -ti -p 1313:1313 -v ~/projects/skia/site2:/input -v /tmp/output/:/output  docsy:latest