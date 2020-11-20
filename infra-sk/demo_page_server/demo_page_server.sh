#!/bin/bash
#
# Runs the demo page server.

CMDLINE_ARGS=$@

# These will be populated/overwritten from command-line flags.
DEMO_PAGE_SERVER_BIN=
HTML_FILE=
PORT=

printUsageAndDie() {
  echo "Usage: $0 <flags>"
  echo ""
  echo "Required flags:"
  echo " -s, --demo-page-server-bin <path>  path to the demo page server bianry"
  echo " -f, --html-file <path>             path to the demo page HTML file"
  echo ""
  echo "Optional flags:"
  echo " -p, --port <port>                  port to serve the pages at (chosen by the OS if unset)"
  exit 1
}

parseFlags() {
  options=$(getopt -u --name $0 \
                   --options s:f:p: \
                   --longoptions demo-page-server-bin:,html-file:,port: \
                   -- ""$CMDLINE_ARGS"")
  if [ $? != "0" ]; then
    printUsageAndDie
  fi
  set -- $options

  while true; do
    case "$1"
    in
      -s|--demo-page-server-bin)
        DEMO_PAGE_SERVER_BIN="$2"; shift;;
      -f|--html-file)
        HTML_FILE="$2"; shift;;
      -p|--port)
        PORT="$2"; shift;;
      --)
        shift; break;;
      *)
        printUsageAndDie;;
    esac
    shift
  done

  # Validate required flags.
  if [[ -z "$DEMO_PAGE_SERVER_BIN" || -z "$HTML_FILE" ]]; then
    printUsageAndDie
  fi
}

main() {
  parseFlags

  # We won't serve the given HTML file directly. Instead, we'll serve its parent directory, which we
  # assume contains the JS/CSS bundles and any other required assets.
  local assets_dir=$(dirname $HTML_FILE)

  # Copy the HTML file as index.html so it is served by default at http://localhost:<port>/.
  cp -f $HTML_FILE $assets_dir/index.html

  # Set the --port flag only if specified.
  PORT_FLAG=
  if [[ ! -z "$PORT" ]]; then
    PORT_FLAG="--port $PORT"
  fi

  # Start the demo page server.
  $DEMO_PAGE_SERVER_BIN --directory $assets_dir $PORT_FLAG
  exit $?
}

main
