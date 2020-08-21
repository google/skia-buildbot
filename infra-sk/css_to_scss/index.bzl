# Naively transforms CSS files into SCSS by transforming CSS "@import" rules into Sass "@use" rules
# and renaming any imported files from *.css to *.scss.
#
# Example input file:
#
#   @import url(./bar.css);
#   @import url(path/to/baz.css);
#   body { color: red; }
#
# Expected output:
#
#   @use "./bar.scss";
#   @use "path/to/baz.scss";
#   body { color: red; }
#
# Note that conditional @imports are not supported (e.g. "@import url(foo.css) screen;").
#
# See the following links for reference:
#  - https://developer.mozilla.org/en-US/docs/Web/CSS/@import
#  - https://sass-lang.com/documentation/at-rules/use
def css_to_scss(name, src, out):
    sed_search_pattern = "@import\s*{location_begin}(.*)\.css{location_end}".format(
        location_begin = "(url\(|'|\\\")",
        location_end = "(\)|'|\\\")")

    sed_replace_pattern = "@use \\\"\\2.scss\\\""

    native.genrule(
        name = name,
        srcs = [src],
        outs = [out],
        cmd = "sed -E \"s/{search}/{replace}/\" $< > $@".format(
            search = sed_search_pattern,
            replace = sed_replace_pattern)
    )
