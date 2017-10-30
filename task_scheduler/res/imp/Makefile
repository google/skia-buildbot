# Set up the local directory to run the demo pages.
default:
	cd ../.. && bower install
	ln -sf ../../third_party/bower_components .
	rm -rf res/
	mkdir -p res/imp
	ln -sfT ../../../../third_party/bower_components res/imp/bower_components
	ln -sfT ../../../../res  res/common
	ln -sfT ../../../res/img res/img
	npm install http-server

# Run a local HTTP server for the demo pages.
run:
	# We download these here because the default target is called by unittests
	wget --no-clobber https://storage.googleapis.com/skia-infra-testdata/task-scheduler-demo/job-search-sk-demo.json
	../../node_modules/.bin/http-server -p 8080 -a $(shell hostname)
