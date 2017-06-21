# This makefile allows to install and run a test server in the root of the site (../..).
# The primary purpose is to drive demo pages for the elements, that use the same root 
# folder as the application in production. 

# default: install the necessary server and creates links for the demo pages to work. 
default:
	ln -sf ../../third_party/bower_components bower_components 
	cd ../.. && bower install
	npm install http-server

# run: runs a local HTTP server for the demo pages, without caching and autoIndex. 
run:
	../../node_modules/.bin/http-server -p 8080 -a 127.0.0.1 -c-1 -i false ../.. 

# runproxy: same as run but proxies unresolved requests to port 9999
runproxy:
	../../node_modules/.bin/http-server -p 8080 -a 127.0.0.1 -c-1 -i false -P http://localhost:9999 ../.. 
