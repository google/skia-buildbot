# Set up the local directory to run the demo pages.
default:
	ln -sf ../../third_party/bower_components bower_components 
	ln -sfr ../../res res
	ln -sfr ../../node_modules node_modules
	bower install
	npm install http-server
# Run a local HTTP server for the demo pages. Make sure to disable caching.
run:
	../../node_modules/.bin/http-server -p 8080 -a 127.0.0.1 -c-1 
