# This makefile allows to install and run a test server in the root of the site (../..).
# The primary purpose is to drive demo pages for the elements, that use the same root
# folder as the application in production.

# default: install the necessary server and creates links for the demo pages to work.
default:
	ln -sf ../../third_party/bower_components bower_components
	cd ../.. && bower install

# run: runs a local HTTP server for the demo pages.
run:
	python -m SimpleHTTPServer 8080

