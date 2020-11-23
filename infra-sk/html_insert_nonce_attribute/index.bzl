"""This module defines the html_insert_nonce_attribute macro."""

def html_insert_nonce_attribute(name, src, out, nonce):
    """Adds a "nonce" attribute to all <link> and <script> tags found in the source HTML file.

    For most Skia Infrastructure apps, the value of nonce attributes is plugged in at run time by
    the Golang binary serving the application. This is typically done using the Golang
    "html/template" package. The examples below use a typical placeholder value found in many of our
    apps.

    # BUILD.bazel
    insert_nonce_attribute(
        name="nonce_generator",
        src="input.html",
        out="output.html",
        nonce="{% .Nonce %}",
    )

    <!-- input.html -->
    <link href="styles.css" rel="stylesheet">
    <script type="text/javascript" src="index.js"></script>

    <!-- output.html -->
    <link nonce="{% .Nonce %}" href="styles.css" rel="stylesheet">
    <script nonce="{% .Nonce %}" type="text/javascript" src="index.js"></script>

    Known limitations: this is a regex-based search/replace operation, unaware of comments or
    strings.

    Args:
      name: Name of the rule.
      src: Label for the input HTML file.
      out: Label for the output HTML file.
      nonce: Contents of the nonce attribute to add to all <link> and <script> tags.
    """
    native.genrule(
        name = name,
        srcs = [src],
        outs = [out],
        cmd = "sed -E 's/(<script|<link)/\\1 nonce=\"%s\"/g' $< > $@" % nonce,
    )
