# Adds a "nonce" attribute to all <link> and <script> tags found in the source HTML file, e.g.:
#
#   # BUILD.bazel
#   insert_nonce_attribute(
#       name="nonce_generator",
#       src="input.html",
#       out="output.html",
#       nonce="ABC123",
#   )
#
#   <!-- input.html -->
#   <link href="styles.css" rel="stylesheet">
#   <script type="text/javascript" src="index.js"></script>
#
#   <!-- output.html -->
#   <link nonce="ABC123" href="styles.css" rel="stylesheet">
#   <script nonce="ABC123" type="text/javascript" src="index.js"></script>
#
# Known limitations: this is a regex-based search/replace operation, unaware of comments or strings.
def html_insert_nonce_attribute(name, src, out, nonce):
    native.genrule(
        name = name,
        srcs = [src],
        outs = [out],
        cmd = "sed -E 's/(<script|<link)/\\1 nonce=\"%s\"/g' $< > $@" % nonce,
    )
