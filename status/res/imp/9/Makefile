# Set up the local directory to run the demo pages.
default:
	bower install
	rm -rf res/
	mkdir --parents res/imp
	ln -sf ../../third_party/bower_components res/imp/bower_components
	ln -sf ../../../../../res res/common
	ln -sf ../../../../res/js res/js

# Run a local HTTP server for the demo pages.
run:
	npm install http-server
	# We use sinon to mock up the demo page's json requests.
	npm install sinon@1.17.2
	cp ../../../node_modules/sinon/pkg/sinon-server-1.17.2.js res/imp/sinon-server-1.17.2.js

	wget --no-clobber https://storage.googleapis.com/skia-infra-testdata/status-demo/commits-table-demo.json
	wget --no-clobber https://storage.googleapis.com/skia-infra-testdata/status-demo/commits-canvas-demo.json
	wget --no-clobber https://storage.googleapis.com/skia-infra-testdata/status-demo/autoroll-widget-demo.json
	wget --no-clobber https://storage.googleapis.com/skia-infra-testdata/status-demo/alert-widget-demo.json

	../../../node_modules/.bin/http-server -p 8080 -a 127.0.0.1
