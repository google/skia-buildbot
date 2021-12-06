include ../make/bazel.mk
include ../kube/kube.mk

build:
	$(BAZEL) build //docsyserver/go/...

test:
	go test -cover ./go/...

release: build
	$(BAZEL) run //docsyserver:push_docsyserver_container

push: pushk release
	pushk --use-temp-checkout docsyserver

# The docker image has two scripts that allow seeing what docs would look like
# when rendered with Hugo/Docsy.
#
# The second renders the docs to an output directory.
render-using-docker-image:
	docker run --entrypoint=/build.sh  -ti -v ~/projects/skia/site:/input -v /tmp/output/:/output  docsyserver:latest

# The second shows a live preview using 'hugo serve'.
preview-using-docker-image:
	docker run --entrypoint=/serve.sh -ti -p 1313:1313 -v ~/projects/skia/site:/input docsyserver:latest

run-local-instance:
	docker run -ti --entrypoint=/usr/local/bin/docsyserver -p 8000:8000 -p 20000:20000 --user root -eGOOGLE_APPLICATION_CREDENTIALS=/creds.json -v $(GOOGLE_APPLICATION_CREDENTIALS):/creds.json docsyserver:latest \
        --work_dir=/tmp \
        --port=:8000 \
        --prom_port=:20000 \
        --doc_path=site \
        --doc_repo=https://skia.googlesource.com/skia \
        --docsy_dir=/home/skia/docsy \
        --hugo=/home/skia/go/bin/hugo \
        --do_login=false
