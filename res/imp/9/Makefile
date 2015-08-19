# Set up the local directory to run the demo pages.
default:
	bower install
	npm install http-server
	mkdir --parents res/imp
	ln -sf ../../bower_components res/imp/bower_components
	ln -sf ../../.. res/common

# Run a local HTTP server for the demo pages.
run:
	node_modules/.bin/http-server
