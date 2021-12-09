"""This module defines the html_insert_assets macro."""

def html_insert_assets(
        name,
        html_src,
        html_out,
        js_src,
        js_serving_path,
        css_src,
        css_serving_path,
        nonce):
    """Inserts <link> and <script> tags into an HTML file to load JavaScript and CSS bundles.

    This macro inserts <link rel="stylesheet" ...> and <script> tags into the given HTML file to
    load the given JavaScript/CSS files from the specified serving paths.

    The js_src and css_src attributes are only used to compute a cache0busting query parameter.

    If the nonce argument is set, a nonce="<value>" tag will be added to both tags.

    Example:

    ```
        # BUILD.bazel
        html_insert_assets(
            name = "production_html",
            html_src = "input.html",
            html_out = "output.html",
            js_src = "bundle.js",
            js_serving_path = "/dist/index.js",
            css_src = "bundle.css",
            css_serving_path = "/dist/index.css",
            nonce = "{% .Nonce %}",
        )
    ```

    ```
        <!-- input.html -->
        <!DOCTYPE html>
        <html>
        <head>
        <title>Example</title>
        </head>
        <body>
        <h1>Hello, world!</h1>
        </body>
        </html>
    ```

    ```
        <!-- output.html -->
        <!DOCTYPE html>
        <html>
        <head>
        <title>Example</title>
        <link rel="stylesheet" href="/dist/index.css?v=3725901259" nonce="{% .Nonce %}"></head>
        <body>
        <h1>Hello, world!</h1>
        <script src="/dist/index.js?v=4277243347" nonce="{% .Nonce %}"></script></body>
        </html>
    ```

    Args:
      name: Name of the rule.
      html_src: Label for the input HTML file.
      html_out: Label for the output HTML file.
      js_src: Label for the JavaScript file. Only used to compute a cache-busting query parameter.
      js_serving_path: Serving path of the JavaScript file, e.g. "/dist/index.js".
      css_src: Label for the CSS file. Only used to compute a cache-busting query parameter.
      css_serving_path: Serving path of the CSS file, e.g. "/dist/index.css".
      nonce: Contents of the nonce attribute of the inserted tags. Optional.
    """

    # Generate <js_src>.version and <css_src>.version files. We will use the contents of these files
    # as the values of the ?v=<value> query parameters that we'll append to the serving paths for
    # cache-busting purposes.
    for src in [js_src, css_src]:
        native.genrule(
            name = src.replace(":", "__") + "_version",
            srcs = [src],
            outs = [src + ".version"],
            # Compute the MD5 hash of the file, and convert it to base 10 using Bash. We can't use
            # the full 128 bits of the hash because Bash overflows at 2^63. To play it extra-safe,
            # we take the 32 most significant bits (i.e. the first 8 characters).
            cmd = "HASH=$$(md5sum $<); echo $$((16#$${HASH:0:8})) > $@",
        )

    native.genrule(
        name = name,
        srcs = [
            html_src,
            js_src + ".version",
            css_src + ".version",
            "//infra-sk/html_insert_assets:html_insert_assets",
        ],
        outs = [html_out],
        cmd = " ".join([
            "$(location //infra-sk/html_insert_assets:html_insert_assets)",
            "--html  $(rootpath %s)" % html_src,
            "--js    %s?v=$$(cat $(location %s))" % (js_serving_path, js_src + ".version"),
            "--css   %s?v=$$(cat $(location %s))" % (css_serving_path, css_src + ".version"),
            "--nonce '%s'" % nonce,
            "> $@",
        ]),
        visibility = ["//visibility:public"],
    )
