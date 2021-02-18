"""This module provides a macro to support Webpack-style SCSS imports for elements-sk styles.

This module will be removed once the migration off Webpack is complete.
"""

def generate_tilde_prefixed_elements_sk_scss_files(name):
    """Copies @infra-sk_npm//:node_modules/elements-sk/**/*.scss files into //bazel-bin/~elemnts-sk.

    This macro adds support for tilde-prefixed Sass imports of elements-sk styles, e.g.:

    ```
    @import '~elements-sk/themes/themes';
    ```

    The output of this macro is a filegroup with the generated tilde-prefixed SCSS files. Do not use
    directly; client code should use the //infra-sk:elements-sk_scss sass_library target instead.

    This hack is necessary to maintain compatibility with the Webpack build, which uses the
    css-loader plugin to inline CSS imports. Said plugin adds all NPM packages to the Sass compiler
    import path, prefixed with "~". See https://webpack.js.org/loaders/css-loader/#import for more.

    This macro will be removed once we're completely off Webpack, and all our SCSS imports have been
    rewritten using the conventional (i.e. non-Webpack) syntax.

    Args:
        name: Name of the output filegroup which will contain the tilde-prefixed SCSS files.
    """

    scss_files = [
        # To regenerate this list, please run the following Bash command:
        #
        #   $ find infra-sk/node_modules/elements-sk -name "*.scss" \
        #       | sed -E "s/infra-sk\//@infra-sk_npm\/\/:/" \
        #       | sort
        "@infra-sk_npm//:node_modules/elements-sk/checkbox-sk/checkbox-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/collapse-sk/collapse-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/colors.scss",
        "@infra-sk_npm//:node_modules/elements-sk/icon/icon-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/multi-select-sk/multi-select-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/nav-links-sk/nav-links-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/radio-sk/radio-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/select-sk/select-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/spinner-sk/spinner-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/styles/buttons/buttons.scss",
        "@infra-sk_npm//:node_modules/elements-sk/styles/select/select.scss",
        "@infra-sk_npm//:node_modules/elements-sk/styles/table/table.scss",
        "@infra-sk_npm//:node_modules/elements-sk/tabs-panel-sk/tabs-panel-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/tabs-sk/tabs-sk.scss",
        "@infra-sk_npm//:node_modules/elements-sk/themes/color-palette.scss",
        "@infra-sk_npm//:node_modules/elements-sk/themes/themes.scss",
        "@infra-sk_npm//:node_modules/elements-sk/toast-sk/toast-sk.scss",
    ]

    tilde_prefixed_scss_files = [
        f.replace("@infra-sk_npm//:node_modules/elements-sk", "~elements-sk")
        for f in scss_files
    ]

    # Copy each SCSS file into its tilde-prefixed location.
    for scss_file, tilde_prefixed_scss_file in zip(scss_files, tilde_prefixed_scss_files):
        native.genrule(
            name = tilde_prefixed_scss_file.replace("/", "__"),  # No slashes on build target names.
            srcs = [scss_file],
            outs = [tilde_prefixed_scss_file],
            cmd = "cp $< $@",
        )

    # Do not use directly. Use //infra-sk:elements-sk_scss instead, which includes these files.
    native.filegroup(
        name = name,
        srcs = tilde_prefixed_scss_files,
        visibility = ["//infra-sk:__pkg__"],
    )
