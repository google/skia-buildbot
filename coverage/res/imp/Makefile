# Set up the local directory to run the demo pages.
default:
	ln -sf ../../res res
	ln -sf ../../node_modules node_modules
	npm install http-server

# Run a local HTTP server for the demo pages.
run:
	../../node_modules/.bin/http-server -p 8080 -a 127.0.0.1
