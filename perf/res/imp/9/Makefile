# Set up the local directory to run the demo pages.
default:
	bower install
	npm install http-server
	mkdir --parents res/imp
	ln -sf ../../third_party/bower_components res/imp/bower_components
	ln -sf ../../../../../res res/common

# Run a local HTTP server for the demo pages.
run:
	http-server
