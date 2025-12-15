"""This module provides the gcs_mirror_url macro."""

load("@bazel_tools//tools/build_defs/repo:http.bzl", "http_archive", "http_file")

# Set to True to force the macro to only return the mirror URL.
_TEST_GCS_MIRROR = False

# Must be kept in sync with the suffixes supported by gcs_mirror (e.g.
# https://skia.googlesource.com/skia/+/8ad66c2340713234df6b249e793415233337a103/bazel/gcs_mirror/gcs_mirror.go#140).
_SUPPORTED_SUFFIXES = [".tar.gz", ".tgz", ".tar.xz", ".deb", ".zip"]

_GCS_MIRROR_PREFIX = "https://cdn.skia.org/bazel"

def gcs_mirror_url(url, sha256, ext = None):
    """Takes the URL of an external resource and computes its GCS mirror URL.

    We store backup copies of external resources in https://cdn.skia.org. This macro
    returns a list with two elements: the original URL, and the mirrored URL.

    To mirror a new URL, please use the `gcs_mirror` utility found at
    https://skia.googlesource.com/skia/+/8ad66c2340713234df6b249e793415233337a103/bazel/gcs_mirror/gcs_mirror.go.

    Args:
        url: URL of the mirrored resource.
        sha256: SHA256 hash of the mirrored resource.
        ext: string matching the extension, if not provided, it will be gleaned from the URL.
             The auto-detected suffix must match a list. An arbitrarily provided one does not.
    Returns:
        A list of the form [original URL, mirror URL].
    """
    extension = ""
    if ext == None:
        for suffix in _SUPPORTED_SUFFIXES:
            if url.endswith(suffix):
                extension = suffix
                break
        if extension == "":
            fail("URL %s has an unsupported suffix." % url)

    mirror_url = "%s/%s%s" % (_GCS_MIRROR_PREFIX, sha256, extension)
    return [mirror_url] if _TEST_GCS_MIRROR else [mirror_url, url]

def _gcs_mirror_impl(ctx):
    def _url_helper(tag):
        # This is a weird effect resulting from the combination of
        # gcs_mirror_url treatment of None vs "" and the fact that
        # attrs.string() cannot be None.
        ext = tag.ext
        if tag.no_extension:
            ext = ""
        elif tag.ext == "":
            ext = None
        return gcs_mirror_url(tag.url, tag.sha256, ext)

    for mod in ctx.modules:
        for tag in mod.tags.http_archive:
            http_archive(
                name = tag.name,
                build_file_content = tag.build_file_content,
                sha256 = tag.sha256,
                strip_prefix = tag.strip_prefix,
                urls = _url_helper(tag),
            )
        for tag in mod.tags.http_file:
            http_file(
                name = tag.name,
                downloaded_file_path = tag.downloaded_file_path,
                executable = tag.executable,
                sha256 = tag.sha256,
                urls = _url_helper(tag),
            )

_http_archive = tag_class(attrs = {
    "name": attr.string(),
    "build_file_content": attr.string(),
    "ext": attr.string(),
    "no_extension": attr.bool(),
    "url": attr.string(),
    "sha256": attr.string(),
    "strip_prefix": attr.string(),
})

_http_file = tag_class(attrs = {
    "name": attr.string(),
    "downloaded_file_path": attr.string(),
    "executable": attr.bool(),
    "ext": attr.string(),
    "no_extension": attr.bool(),
    "sha256": attr.string(),
    "url": attr.string(),
})

gcs_mirror = module_extension(
    doc = """Bzlmod extension wrapper around gcs_mirror_url.""",
    implementation = _gcs_mirror_impl,
    tag_classes = {
        "http_archive": _http_archive,
        "http_file": _http_file,
    },
)
