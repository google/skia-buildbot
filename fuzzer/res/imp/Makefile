# Set up the local directory to run the demo pages.
default:
	bower install
	npm install http-server
	npm install sinon@1.17.2
	rm -rf res/
	mkdir --parents res/imp
	ln -sf ../../third_party/bower_components res/imp/bower_components
	ln -sf ../../../../res res/common
	ln -sf ../../../res/js res/js
	cp ../../node_modules/sinon/pkg/sinon-server-1.17.2.js res/imp/sinon-server-1.17.2.js

# Run a local HTTP server for the demo pages.
run:
	../../node_modules/.bin/http-server -p 8080 -a 127.0.0.1