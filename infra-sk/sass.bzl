"""This module defines the sass_library and sass_binary rules."""

load("@aspect_bazel_lib//lib:copy_to_bin.bzl", "copy_to_bin")
load("@npm//:csso-cli/package_json.bzl", _csso_bin = "bin")
load("@npm//:sass/package_json.bzl", _sass_bin = "bin")

def sass_library(name, srcs, visibility, deps = []):
    """Groups together one or more .scss or .css files.

    This rule simply copies its input files to the Bazel output tree with the copy_to_bin rule
    (https://docs.aspect.build/rulesets/aspect_bazel_lib/docs/copy_to_bin). The reason we use this
    rule rather than a filegroup is that the js_binary rules automatically generated[1] by rules_js
    are not designed to work with files outside of the Bazel package where they are defined, and
    doing so produces errors such as:

        Expected to find file foo.scss in //my/package, but instead it is in //another/package.

    [1] https://docs.aspect.build/rulesets/aspect_rules_js/docs/#using-binaries-published-to-npm.

    Args:
      name: Name of the target.
      srcs: Sass source files (either .scss or .css files).
      visibility: Visibility of the target.
      deps: Any sass_library dependencies.
    """

    copy_to_bin(
        name = name,
        srcs = srcs + deps,
        visibility = visibility,
    )

def sass_binary(name, srcs, entry_point, out, mode, deps = []):
    """Compiles Sass stylesheets into CSS.

    This macro is a simple wrapper around the Sass compiler (https://www.npmjs.com/package/sass).
    In addition to compiling to CSS, this macro optimizes the resulting CSS with CSSO
    (https://www.npmjs.com/package/csso-cli). This eliminates any repeated rules that might result
    from including the same Sass file multiple times.

    When the mode argument is set to "development", the resulting .css file will be unminified and
    will include an embedded sourcemap. When the mode argument is set to "production", no sourcemap
    will be produced, and the resulting file will be minified.

    Args:
      name: Name of the target.
      srcs: Sass source files (either .scss or .css files).
      entry_point: A single Sass file that will be passed to the Sass compiler as the entry point.
      out: Name of the output .css file.
      mode: Either "development" or "production".
      deps: Any sass_library dependencies.
    """

    if entry_point not in srcs:
        fail("The entry_point must be included in the srcs.")

    if mode not in ["development", "production"]:
        fail("Unknown value for \"mode\" argument: \"%s\"." % mode)

    # Reference: https://sass-lang.com/documentation/cli/dart-sass.
    sass_args = [
        "--load-path=.",
    ]
    if mode == "development":
        sass_args += [
            "--style=expanded",
            # Ideally we would want to use --embed-sources and --embed-source-map simultaneously so
            # that we only get the .css file as output, but for some reason the embedded sourcemap
            # does not work in Chrome and makes csso crash. We work around this by omitting
            # --embed-source-map and passing the resulting .css.map file to csso.
            "--embed-sources",
        ]
    else:
        sass_args += [
            "--style=compressed",
            "--no-source-map",
        ]

    out_unoptimized = name + "_unoptimized.css"

    # See https://docs.aspect.build/rulesets/aspect_rules_js/docs/#using-binaries-published-to-npm.
    _sass_bin.sass(
        name = name + "_unoptimized",
        srcs = srcs + deps,
        args = sass_args + [
            "$(rootpath %s)" % entry_point,
            "$(rootpath %s)" % out_unoptimized,
        ],
        outs = [out_unoptimized] + ([out_unoptimized + ".map"] if mode == "development" else []),
    )

    # Reference: https://github.com/css/csso-cli.
    csso_args = []
    if mode == "development":
        csso_args = [
            "--input-source-map",
            "$(rootpath %s.map)" % out_unoptimized,
            "--source-map",
            "inline",
        ]
    else:
        csso_args = [
            "--input-source-map",
            "none",
            "--source-map",
            "none",
        ]

    # See https://docs.aspect.build/rulesets/aspect_rules_js/docs/#using-binaries-published-to-npm.
    _csso_bin.csso(
        name = name,
        srcs = [out_unoptimized] + ([out_unoptimized + ".map"] if mode == "development" else []),
        args = [
            "$(rootpath %s)" % out_unoptimized,
            "--output",
            "$(rootpath %s)" % out,
        ] + csso_args,
        outs = [out],
        visibility = ["//visibility:public"],
    )
