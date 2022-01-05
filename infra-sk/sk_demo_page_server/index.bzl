"""This module defines the sk_demo_page_server macro."""

_demo_page_server = "//infra-sk/sk_demo_page_server:demo_page_server"

_demo_server_script_template = """#!/bin/bash
#
# Runs the demo page server.
set -x
HTML_FILE=$(rootpath {html_file})
DEMO_PAGE_SERVER_BIN=$(rootpath {demo_page_server})
PORT={port}

# We won't serve the given HTML file directly. Instead, we'll serve its parent directory, which we
# assume contains the JS/CSS bundles and any other required assets.
ASSETS_DIR=$$(dirname $$HTML_FILE)

# Copy the HTML file as index.html so it is served by default at http://localhost:<port>/.
cp -f $$HTML_FILE $$ASSETS_DIR/index.html

# Copy any static assets into the serving directory.
{copy_static_assets}

# Start the demo page server.
$$DEMO_PAGE_SERVER_BIN --directory $$ASSETS_DIR --port $$PORT
exit $$?
"""

def sk_demo_page_server(name, sk_page, static_assets = None, port = 8080):
    """Creates a demo page server for the given page.

    This target can be used during development with "bazel run".

    It can also be used as the environment for a test_on_env test (e.g. for Puppeteer tests). The
    TCP port for the HTTP server can be found in $ENV_DIR/port.

    Args:
      name: Name of the rule.
      sk_page: Label of the sk_page to serve.
      static_assets: A dictionary where the keys are serving paths (e.g. "/static/img"), and the
        values are a list of Bazel labels of the files that should be served in the path (e.g.
        ["//myapp/assets/logo.png". "//myapp/assets/favicon.ico"]).
      port: Port for the HTTP server. Set to 0 to let the OS choose an unused port.
    """

    copy_static_assets = "\n".join([
        "mkdir -p $$ASSETS_DIR%s && cp -f $(rootpath %s) $$ASSETS_DIR%s" % (dir, file, dir)
        for dir in static_assets
        for file in static_assets[dir]
    ]) if static_assets else ""

    script = _demo_server_script_template.format(
        html_file = sk_page + "_html_dev",
        copy_static_assets = copy_static_assets,
        demo_page_server = _demo_page_server,
        port = port,
    )

    static_assets_labels = [
        file
        for dir in static_assets
        for file in static_assets[dir]
    ] if static_assets else []

    native.genrule(
        name = name + "_genrule",
        srcs = [
            # Label for the development .html file produced by the sk_page rule. We need to list
            # this explicitly in the srcs attribute in order for the $(rootpath {html_file})
            # variable expansion to work.
            sk_page + "_html_dev",
            _demo_page_server,
        ] + static_assets_labels,
        outs = [name + ".sh"],
        cmd = "echo '{}' > $@".format(script.replace("'", "'\"'\"'")),
        executable = 1,
    )

    native.sh_binary(
        name = name,
        srcs = [name + "_genrule"],
        data = [
            _demo_page_server,
            # This label includes the development .html file produced by the sk_page rule and all
            # its assets (JS/CSS development bundles, etc.).
            sk_page + "_dev",
        ] + static_assets_labels,
    )
