# Set up the local directory to run the demo pages.
default:
	bower install
	rm -rf res/
	mkdir --parents res/imp
	ln -sf ../../third_party/bower_components res/imp/bower_components
	ln -sf ../../../../res res/common
	ln -sf ../../../res/js res/js
	ln -sf ../../../res/img res/img

# Run a local HTTP server for the demo pages.
run:
	npm install http-server
	# We use sinon to mock up the demo page's json requests.
	npm install sinon@1.17.2
	cp ../../node_modules/sinon/pkg/sinon-1.17.2.js res/imp/sinon-1.17.2.js
	# Start the server
	../../node_modules/.bin/http-server -p 8080 -a 127.0.0.1