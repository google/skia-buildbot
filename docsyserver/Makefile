include ../make/bazel.mk

build:
	$(BAZEL) build --config=mayberemote //docsyserver/go/...

build-local-image: build
	$(BAZEL) run --config=mayberemote //docsyserver:docsyserver_container

test:
	go test -cover ./go/...

release: build
	$(BAZEL) run --config=mayberemote //docsyserver:push_docsyserver_container

push: release
	$(BAZEL) run --config=mayberemote //kube/go/pushk -- --use-temp-checkout docsyserver

# The docker image has two scripts that allow seeing what docs would look like
# when rendered with Hugo/Docsy.
#
# The second renders the docs to an output directory.
render-using-docker-image:
	docker run --entrypoint=/build.sh  -ti -v ~/projects/skia/site:/input -v /tmp/output/:/output bazel/docsyserver:docsyserver_container

# The second shows a live preview using 'hugo serve'.
preview-using-docker-image:
	docker run --entrypoint=/serve.sh -ti -p 1313:1313 -v ~/projects/skia/site:/input bazel/docsyserver:docsyserver_container

run-local-instance:
	docker run -ti --entrypoint=/usr/local/bin/docsyserver -p 8000:8000 -p 20000:20000 --user root -eGOOGLE_APPLICATION_CREDENTIALS=/creds.json -v $(GOOGLE_APPLICATION_CREDENTIALS):/creds.json bazel/docsyserver:docsyserver_container \
        --work_dir=/tmp \
        --port=:8000 \
        --prom_port=:20000 \
        --doc_path=site \
        --doc_repo=https://skia.googlesource.com/skia \
        --docsy_dir=/home/skia/docsy \
        --hugo=/home/skia/go/bin/hugo
