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
# TODO(borenet): Some symlinks are missing below.
$(httpserver):
	ln -sf ../../third_party/bower_components bower_components
	if [ ! -f bower.json ]; then cd ../..; fi && bower install
	npm install http-server

# Run a local HTTP server for the demo pages.
.PHONY: run
run: $(httpserver)
	$(httpserver) -p $(port) -a $(shell hostname)

.PHONY: echo
echo:
	echo $(server)
	echo $(port)
	echo $(httpserver)
