build:
	go install ./go/...

build-local-image: build
	SKIP_UPLOAD=1 ./images/release

test:
	go test ./go/...

release: build
	./images/release

# The docker image has two scripts that allow seeing what docs would look like
# when rendered with Hugo/Docsy.
#
# The first shows a live preview using 'hugo serve'.
preview-using-docker-image:
	docker run --entrypoint=/serve.sh  -ti -p 1313:1313 -v ~/projects/skia/site2:/input -v /tmp/output/:/output  docsy:latest

# The second renders the docs to an output directory.
render-using-docker-image:
	docker run --entrypoint=/build.sh -ti -p 1313:1313 -v ~/projects/skia/site2:/input docsy:latest

