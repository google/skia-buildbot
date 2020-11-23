"""This module defines the remove_css_imports_from_js macro."""

def remove_css_imports_from_js(name, src, out):
    """Removes CSS files imported from a JavaScript file via a require() call.

    Useful for removing CSS imports from UMD bundles generated with npm_umd_bundle(), which
    typically generate errors when loaded from a karma_mocha_test() or a karma_web_test().

    This macro should be deleted once we are completely off Webpack and all our Sass/CSS imports
    from TypeScript code have been removed.

    Args:
      name: Name of the rule.
      src: Label for the input JavaScript file.
      out: Label for the output JavaScript file.
    """
    native.genrule(
        name = name,
        srcs = [src],
        outs = [out],
        cmd = "sed 's/^[ \t]*require(.*\\.[s]\\?css.);\\?//' $< > $@",
    )
