build:
	go install ./go/...

build-local-image: build
	SKIP_UPLOAD=1 ./images/release

test:
	go test -cover ./go/...

release: build
	./images/release

push: release
	pushk docsyserver

# The docker image has two scripts that allow seeing what docs would look like
# when rendered with Hugo/Docsy.
#
# The second renders the docs to an output directory.
render-using-docker-image:
	docker run --entrypoint=/build.sh  -ti -p 1313:1313 -v ~/projects/skia/site2:/input -v /tmp/output/:/output  docsyserver:latest

# The second shows a live preview using 'hugo serve'.
preview-using-docker-image:
	docker run --entrypoint=/serve.sh -ti -p 1313:1313 -v ~/projects/skia/site2:/input docsyserver:latest

run-local-instance:
	docker run -ti --entrypoint=/usr/local/bin/docsyserver -p 8000:8000 -p 20000:20000 --user root -eGOOGLE_APPLICATION_CREDENTIALS=/creds.json -v $(GOOGLE_APPLICATION_CREDENTIALS):/creds.json docsyserver:latest \
        --work_dir=/tmp \
        --port=:8000 \
        --prom_port=:20000 \
        --doc_path=site2 \
        --doc_repo=https://skia.googlesource.com/skia \
        --docsy_dir=/home/skia/docsy \
        --hugo=/home/skia/go/bin/hugo \
        --do_login=false \
        --logtostderr