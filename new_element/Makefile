default: ${GOPATH}/bin/rice rice-box.go
	go install .

rice-box.go: $(wildcard ./templates/*)
	go generate .
	rice embed-go

# The command line tool that embeds plain files into Go code.
${GOPATH}/bin/rice:
	go get github.com/GeertJohan/go.rice/rice

.PHONY: debug
debug:
	npx webpack-cli --mode=development

.PHONY: testjs
testjs:
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

.PHONY: test-frontend-ci
test-frontend-ci: build-frontend-ci
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

.PHONY: build-frontend-ci
build-frontend-ci:
	npm ci
	npx webpack --mode=development

.PHONY: testpptr
testpptr:
	NODE_OPTIONS="--max-old-space-size=$(NODE_MEMORY_LIMIT)" npx mocha -r ts-node/register ./modules/**/*_puppeteer_test.ts