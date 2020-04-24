core:
	go install -v ./go/demosfe

serve:
	npx webpack-dev-server --watch-poll --mode-development

build_resources:
	npx webpack --mode=production

run: build_resources core
	demosfe --alsologtostderr

release: build_resources
	CGO_ENABLED=0 GOOS=linux go install -a ./go/demosfe
	./build_release
