"""
Provides the oci_extract rule, used for extracting files from an oci_image.
"""

def oci_extract(name, image, paths, **kwargs):
    """Extract files from an OCI image.

    Args:
        name: Name of the target.
        image: Label of the image to use.
        paths: Dictionary whose keys are absolute paths within the image and
            values are extraction destination paths.
        **kwargs: Additional arguments to pass to the genrule.
    """
    python = "@python_3_11//:py3_runtime"
    script = "//bazel:oci-extract.py"
    cmd = """
PYTHON=""
for path in $(locations {python}); do
    if [[ $$path = *"/bin/python3" ]]; then
        PYTHON="$$path"
    fi
done

$$PYTHON $(execpath {script}) --input=$(execpath {image}) --dest=$(RULEDIR) {path_flags}
""".format(
        python = python,
        script = script,
        image = image,
        path_flags = " ".join(["--path=%s:%s" % (input, output) for input, output in paths.items()]),
    )
    native.genrule(
        name = name,
        srcs = [image],
        outs = paths.values(),
        cmd = cmd,
        tools = [
            python,
            script,
        ],
        **kwargs
    )
