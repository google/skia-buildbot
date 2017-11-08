.PHONY: default
default: run

# Find the name of the server based on our CWD.
# Assume we're in a subdirectory of $GOPATH/src/go.skia.org/infra, trim
# everything up to and including "infra", plus any "/res/imp" suffix. For most
# servers that gets us the server name. Some others also have a "frontend"
# subdir, which we also trim. For the common "res/imp" directory, we just use
# that.
server = $(shell pwd | sed 's/.*infra\///' | sed 's/\/\(frontend\/\)\?res\/imp//')

# Derive a 4-digit port number starting with '8' by hashing the server name.
port = 8$(shell printf "%03d" $(shell expr $(shell cksum <<< "$(server)" | cut -f 1 -d ' ') % 999))

# Determine where http-server should be.
httpserver_dir = $(shell if [ -f bower.json ]; then echo "."; else echo "../.."; fi)
httpserver = $(httpserver_dir)/node_modules/.bin/http-server

# Set up the local directory to run the demo pages.
$(httpserver): ../../node_modules bower_components res/imp/bower_components res/common res/js
	npm install http-server

bower_components: ../../third_party/bower_components
	ln -sf ../../third_party/bower_components bower_components

../../third_party/bower_components:
	cd ../.. && bower install

../../node_modules:
	cd ../.. && bower install

res:
	mkdir -p res

res/imp: res
	mkdir res/imp

res/imp/bower_components: res/imp
	ln -sfT ../../../../third_party/bower_components res/imp/bower_components

res/common:
	ln -sfT ../../../../res  res/common

res/js:
	ln -sfT ../../../res/js res/js

# Run a local HTTP server for the demo pages.
.PHONY: run
run: $(httpserver)
	$(httpserver) -p $(port) -a $(shell hostname)

.PHONY: echo
echo:
	echo $(server)
	echo $(port)
	echo $(httpserver)
