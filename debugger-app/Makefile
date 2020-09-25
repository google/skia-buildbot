KGO := CGO_ENABLED=0 GOOS=linux go build

.PHONY: test
test: go-test js-test
	true

.PHONY: go-test
go-test:
	go test ./go/...

.PHONY: js-test
js-test:
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

.PHONY: js-test-debug
js-test-debug:
	npx karma start --no-single-run

node_modules: package.json
	npm ci

.PHONY: serve
serve:
	npx webpack-dev-server --watch-poll --mode=development