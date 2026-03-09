"""This module defines the "docsy" module extension.

It downloads docsy and runs "npm install".
"""

node_version = "v14.16.0"

def _clone_and_npm_install_impl(repository_ctx):
    def run(cmd):
        res = repository_ctx.execute(cmd)
        if res.return_code != 0:
            fail("%s failed: %s" % (" ".join(cmd), res.stderr))

    # Clone the docsy example repo.
    run(["git", "init"])
    run(["git", "remote", "add", "origin", repository_ctx.attr.remote])
    run(["git", "fetch", "--depth=1", "origin", repository_ctx.attr.commit])
    run(["git", "checkout", repository_ctx.attr.commit])
    run(["git", "submodule", "update", "--init", "--recursive"])

    # Download Node at a specific version known to work.
    node_url = "https://nodejs.org/dist/" + node_version + "/node-" + node_version + "-linux-x64.tar.gz"
    repository_ctx.download_and_extract(
        url = node_url,
        output = "node",
        strip_prefix = "node-" + node_version + "-linux-x64",
    )
    node_bin = repository_ctx.path("node/bin")
    npm_bin = repository_ctx.path("node/bin/npm")

    # Run "npm install".
    res = repository_ctx.execute(
        [npm_bin, "install"],
        environment = {
            "NODE_VERSION": node_version,
            "PATH": str(node_bin) + ":/usr/bin",
        },
        quiet = False,
    )
    if res.return_code != 0:
        fail("npm install failed: %s" % res.stderr)

    # Delete unwanted files; this is an example repo and we're replacing these
    # files with configuration and content of our own.
    repository_ctx.delete("config.toml")
    repository_ctx.delete("content")

    # Create a tarball of the whole directory to preserve symlinks.
    # We want the contents to be at /home/skia/docsy in the container.
    run(["mkdir", "-p", "home/skia/docsy"])

    # Copy all files and folders into home/skia/docsy, avoiding infinite recursion into home
    # by using find with a maxdepth of 1 and skipping the '.' and 'home' directories.
    # We use cp -a to preserve permissions and symlinks.
    run(["sh", "-c", "find . -maxdepth 1 -mindepth 1 ! -name home -exec cp -a {} home/skia/docsy/ \\;"])

    # Ensure all files and directories under home/skia/docsy are readable and executable
    # so that the docsyserver container can run node/npm/npx correctly.
    run(["chmod", "-R", "a+rx", "home/skia/docsy"])

    run(["tar", "-cf", "docsy.tar", "home/skia/docsy"])

    # Add a BUILD.bazel file to export the tarball.
    repository_ctx.file("BUILD.bazel", """
exports_files(["docsy.tar"])
""")

_clone_and_npm_install = repository_rule(
    implementation = _clone_and_npm_install_impl,
    attrs = {
        "remote": attr.string(mandatory = True),
        "commit": attr.string(mandatory = True),
    },
)

def _docsy_impl(_):
    _clone_and_npm_install(
        name = "docsy",
        remote = "https://github.com/google/docsy-example",
        commit = "70e301f7861122ab129d2c46ee5ed625e92c04d0",
    )

docsy = module_extension(implementation = _docsy_impl)
