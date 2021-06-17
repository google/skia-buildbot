"""This module defines the skia_app_container macro."""

load("@io_bazel_rules_docker//container:container.bzl", "container_image", "container_push")
load("@rules_pkg//:pkg.bzl", "pkg_tar")

def skia_app_container(name, repository, dirs, entrypoint):
    """Builds a Docker container for a Skia app, and generates a target to push it to GCR.

    This macro produces a "<name>" target to build the Docker container, and a "push_<name>" target
    to push the container to GCR.

    Example:

    ````
        # //myapp/BUILD.bazel

        load("//bazel:skia_app_container.bzl", "skia_app_container")

        skia_app_container(
            name = "myapp_container",
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
    ````

    The above example will produce a Docker container based on gcr.io/skia-public/basealpine with
    the following contents:

      - /usr/local/bin/myapp/mybinary (mode: 755)
      - /usr/local/share/myapp/config.cfg (mode: 644)
      - /usr/local/share/myapp/data.json (mode: 644)

    To build the container and load it into Docker:

    ````
        $ bazel run //myapp:myapp_container
        ...
        Loaded image ID: sha256:c0decafe
        Tagging c0decafe as bazel/myapp:myapp_container
    ````

    To debug the container locally:

    ```
        $ docker run bazel/myapp:myapp_container
        $ docker run -it --entrypoint /bin/sh bazel/myapp:myapp_container
    ```

    To push the container to GCR:

    ```
        $ bazel run //myapp:push_myapp_container
        ...
        Successfully pushed Docker image to gcr.io/skia-public/myapp:...
    ```

    To push the app to production (assuming the app is pushk-enabled):

    ```
        $ bazel run //myapp:push_myapp_container
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

    container_image(
        name = name,
        base = "@basealpine//image",
        entrypoint = [entrypoint],
        stamp = True,
        tars = pkg_tars,
        user = "skia",
    )

    container_push(
        name = "push_" + name,
        format = "Docker",
        image = name,
        registry = "gcr.io",
        repository = repository,
        tag = "{STABLE_DOCKER_TAG}",
        tags = [
            "manual",  # Exclude it from wildcard queries, e.g. "bazel build //...".
            "no-remote",  # We cannot build :rbe_container_skia_infra on RBE.
        ],
    )
