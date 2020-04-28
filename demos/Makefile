core:
	go install -v ./go/demoserver

serve:
	npx webpack-dev-server --watch-poll --mode-development

fetch_demos:
	go run ./go/load-demo-assets/main.go \
		--alsologtostderr \
		--demos_dir "demos/internal" \
		--out_dir "../../fetched_demos" \
		--repo_url "https://skia.googlesource.com/infra-internal"

build_resources:
	npx webpack --mode=production

run: build_resources core
	demoserver --alsologtostderr

release: build_resources
	CGO_ENABLED=0 GOOS=linux go install -a ./go/demoserver
	./build_release
