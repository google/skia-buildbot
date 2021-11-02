"""This module defines the skia_app_container macro."""

load("@io_bazel_rules_docker//container:container.bzl", "container_image", "container_push")
load("@io_bazel_rules_docker//docker/util:run.bzl", "container_run_and_commit")
load("@rules_pkg//:pkg.bzl", "pkg_tar")

def skia_app_container(
        name,
        repository,
        dirs,
        entrypoint = "",
        run_commands_root = None,
        base_image = "@basealpine//image"):
    """Builds a Docker container for a Skia app, and generates a target to push it to GCR.

    This macro produces the following:
    * "<name>" target to build the Docker container with skia as default user.
    * "<name>_run_root" target to execute run commands as root on the image.
                        root will be the default user here. Will be created only
                        if run_commands_root is specified.
    * "push_<name>" target to push the container to GCR.
    * "pushk_<name>" target to push the container to GCR, and deploy <name> to production via pushk.

    Example:

    ```
        # //myapp/BUILD.bazel

        load("//bazel:skia_app_container.bzl", "skia_app_container")

        skia_app_container(
            name = "myapp",
            dirs = {
                "/usr/local/bin/myapp": [
                    ["//myapp/go:mybinary", 755"],
                ],
                "/usr/local/share/myapp": [
                    ["//myapp/config:config.cfg", "644"],
                    ["//myapp/data:data.json", "644"],
                ],
            },
            entrypoint = "/usr/local/bin/myapp/mybinary",
            repository = "skia-public/myapp",
        )
    ```

    The above example will produce a Docker container based on gcr.io/skia-public/basealpine with
    the following contents:

      - /usr/local/bin/myapp/mybinary (mode: 755)
      - /usr/local/share/myapp/config.cfg (mode: 644)
      - /usr/local/share/myapp/data.json (mode: 644)

    To build the container and load it into Docker:

    ```
        $ bazel run //myapp:myapp
        ...
        Loaded image ID: sha256:c0decafe
        Tagging c0decafe as bazel/myapp:myapp
    ```

    To debug the container locally:

    ```
        $ docker run bazel/myapp:myapp
        $ docker run -it --entrypoint /bin/sh bazel/myapp:myapp
    ```

    To push the container to GCR:

    ```
        $ bazel run //myapp:push_myapp
        ...
        Successfully pushed Docker image to gcr.io/skia-public/myapp:...
    ```

    To push the app to production (assuming the app is compatible with pushk):

    ```
        $ bazel run //myapp:pushk_myapp
    ```

    Which is equivalent to:

    ```
        $ bazel run //myapp:push_myapp
        $ pushk myapp
    ```

    Args:
      name: Name of the rule.
      repository: Name of the repository under gcr.io.
      dirs: Contents of the container, expressed as a dictionary where the keys are directory names
        within the container (e.g. "/usr/local/share/myapp"), and the values are an array of
        [Bazel label, mode] tuples indicating which files should be copied into the directory (e.g.
        ["//myapp/go:mybinary", "755"]).
      entrypoint: The entrypoint of the container, which can be a string or an array (e.g.
        "/usr/local/share/myapp/mybinary", or ["/usr/local/share/myapp/mybinary", "--someflag"]).
        Optional.
      run_commands_root: The RUN commands that should be executed on the container by the root
        user. Optional.
      base_image: The image to base the container_image on. Optional.
    """

    # According to the container_image rule's docs[1], the recommended way to place files in
    # specific directories is via the pkg_tar rule.
    #
    # The below loop creates one pkg_tar rule for each file in the container.
    #
    # [1] https://github.com/bazelbuild/rules_docker/blob/454981e65fa100d37b19210ee85fedb2f7af9626/README.md#container_image
    pkg_tars = []
    i = 0
    for dir in dirs:
        for file, mode in dirs[dir]:
            pkg_tar_name = name + "_pkg_tar_" + str(i)
            i += 1
            pkg_tars.append(pkg_tar_name)

            pkg_tar(
                name = pkg_tar_name,
                srcs = [file],
                package_dir = dir,
                mode = mode,
            )

    image_name = (name + "_base") if run_commands_root else name

    container_image(
        name = image_name,
        base = base_image,

        # We cannot use an entrypoint with the container_run_and_commit rule
        # required when run_commands_root is specified, because the commands we
        # want to execute do not require a specific entrypoint.
        # We will set the entrypoint back after the container_run_and_commit
        # rule is executed.
        entrypoint = None if run_commands_root else [entrypoint],
        stamp = True,
        tars = pkg_tars,
        user = "skia",
    )

    if run_commands_root:
        rule_name = name + "_run_root"
        container_run_and_commit(
            name = rule_name,
            commands = run_commands_root,
            docker_run_flags = ["--user", "root"],
            image = image_name + ".tar",
            tags = [
                # container_run_and_commit requires the docker daemon to be
                # running. This is not possible inside RBE.
                "no-remote",
            ],
        )
        image_name = ":" + rule_name + "_commit.tar"

        # The above container_run_and_commit sets root as the default user and
        # overrides the entrypoint.
        # Now execute container_image using the previous image as base to set
        # back skia as the default user and to set back the original entrypoint.
        rule_name = name
        container_image(
            name = rule_name,
            base = image_name,
            entrypoint = [entrypoint],
            stamp = True,
            tars = pkg_tars,
            user = "skia",
        )
        image_name = ":" + rule_name

    container_push(
        name = "push_" + name,
        format = "Docker",
        image = image_name,
        registry = "gcr.io",
        repository = repository,
        tag = "{STABLE_DOCKER_TAG}",
        tags = [
            "manual",  # Exclude it from wildcard queries, e.g. "bazel build //...".
            # container_push requires the docker daemon to be
            # running. This is not possible inside RBE.
            "no-remote",
        ],
    )

    # The container_push rule outputs two files: <name>, which is a script that uploads the
    # container to GCR, and <name>.digest, which contains the SHA256 digest of the container.
    #
    # Because the container_push rule outputs multiple files, we cannot use $$(rootpath push_<name>)
    # to get the path to <name>, so we use $$(rootpaths push_<name>), which returns the list of all
    # output files, then take the base directory of an arbitrary file, and append <name> to it to
    # get the path to the desired script.
    pushk_script = "\n".join([
        "container_push_outputs=($(rootpaths push_%s))",
        "container_push_base_dir=$$(dirname $${container_push_outputs[0]})",
        "container_push_script=$${container_push_base_dir}/push_%s",
        "",
        "$$container_push_script && $(rootpath //kube/go/pushk) %s",
    ]) % (name, name, name)

    native.genrule(
        name = "gen_pushk_" + name,
        srcs = [
            "push_" + name,
            "//kube/go/pushk",
        ],
        outs = ["pushk_%s.sh" % name],
        cmd = "echo '%s' > $@" % pushk_script,
    )

    native.sh_binary(
        name = "pushk_" + name,
        srcs = ["gen_pushk_" + name],
        data = [
            "push_" + name,
            "//kube/go/pushk",
        ],
    )
