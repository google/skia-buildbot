default: build package-lock.json
	npx webpack --mode=development

build:
	go install ./go/named-fiddles

release: build package-lock.json
	npx webpack --mode=production
	CGO_ENABLED=0 GOOS=linux go install -a ./go/named-fiddles
	./build_release

push: release
	pushk named-fiddles

serve: package-lock.json
	npx webpack-dev-server --mode=development --watch

test:
	# Run the generated tests just once under Xvfb.
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run

testci:
	rm -rf node_modules
	npm install
	xvfb-run --auto-servernum --server-args "-screen 0 1280x1024x24" npx karma start --single-run --no-colors

watch: package-lock.json
	npx webpack --mode=development --watch

package-lock.json: package.json
	npm install
