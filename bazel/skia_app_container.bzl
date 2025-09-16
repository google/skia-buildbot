"""This module defines the skia_app_container macro."""

def skia_app_container(
        name,
        repository,
        dirs,
        entrypoint = "",
        run_commands_root = None,
        run_commands_skia = None,
        base_image = "@basealpine//image",
        env = None,
        default_user = "skia"):
    """Builds a Docker container for a Skia app, and generates a target to push it to GCR.

    This macro produces the following:
    * "<name>" target to build the Docker container with skia as default user.
    * "<name>_run_root" target to execute run commands as root on the image.
                        root will be the default user here. Will be created only
                        if run_commands_root is specified.
    * "<name>_run_skia" target to execute run commands as the "skia" user on the image.
                        Will be created only if run_commands_skia is specified.
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
      run_commands_skia: The RUN commands that should be executed on the container by the skia
        user. Optional.
      base_image: The image to base the container_image on. Optional.
      env: A {"var": "val"} dictionary with the environment variables to use when building the
        container. Optional.
      default_user: The user the container will be run with. Defaults to "skia" but some apps
        like skfe requires the default user to be "root".
    """

    # According to the container_image rule's docs[1], the recommended way to place files in
    # specific directories is via the pkg_tar rule.
    #
    # The below loop creates one pkg_tar rule for each file in the container.
    #
    # [1] https://github.com/bazelbuild/rules_docker/blob/454981e65fa100d37b19210ee85fedb2f7af9626/README.md#container_image
    pass
