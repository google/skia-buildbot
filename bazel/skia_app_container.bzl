"""This module defines the skia_app_container macro."""

load("@io_bazel_rules_docker//container:container.bzl", "container_image", "container_push")
load("@rules_distroless//distroless:defs.bzl", "group", "home", "passwd")
load("@rules_pkg//:pkg.bzl", "pkg_tar")
load(
    "//bazel:owners_layers.bzl",
    "ROOT_GID",
    "ROOT_UID",
    "ROOT_USERNAME",
    "SKIA_GID",
    "SKIA_UID",
    "SKIA_USERNAME",
    "get_fixup_owners_layers",
)

def skia_app_container(
        name,
        repository,
        dirs,
        empty_dirs = None,
        entrypoint = "",
        base_image = "@basealpine//image",
        env = None,
        create_skia_user = False,
        default_user = "skia",
        extra_tars = None,
        owners = None):
    """Builds a Docker container for a Skia app, and generates a target to push it to GCR.

    This macro produces the following:
    * "<name>" target to build the Docker container with skia as default user.

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

    Args:
      name: Name of the rule.
      repository: Name of the repository under gcr.io.
      dirs: Contents of the container, expressed as a dictionary where the keys are directory names
        within the container (e.g. "/usr/local/share/myapp"), and the values are an array of
        [Bazel label, mode] tuples indicating which files should be copied into the directory (e.g.
        ["//myapp/go:mybinary", "755"]).
      empty_dirs: Mapping of directory paths to file modes of empty directories to create.
      entrypoint: The entrypoint of the container, which can be a string or an array (e.g.
        "/usr/local/share/myapp/mybinary", or ["/usr/local/share/myapp/mybinary", "--someflag"]).
        Optional.
      base_image: The image to base the container_image on. Optional.
      env: A {"var": "val"} dictionary with the environment variables to use when building the
        container. Optional.
      create_skia_user: Whether or not to create the "skia" user with uid 2000 and gid 2000.
      default_user: The user the container will be run with. Defaults to "skia" but some apps
        like skfe requires the default user to be "root".
      extra_tars: A list of target names of tarballs to be added to the image.
      owners: Optional. A dictionary where keys are absolute directory paths within the
        container (e.g., "/home/skia"), and values are the desired owner in "uid.gid"
        format (e.g., "2000.2000"). The macro will ensure these directories and their
        subdirectories (created via 'dirs') have the specified ownership.
    """

    if type(entrypoint) == "string":
        entrypoint = [entrypoint]

    # Derive the ownership fixup layers. We'll use them to set ownership for
    # 'dirs' and then fixup directory owners later. See documentation in
    # owners_layers.bzl for more information.
    fixup_owners_layers = get_fixup_owners_layers(dirs.keys(), owners or {})
    owners_lookup = {}
    for layer in fixup_owners_layers:
        for path in layer.paths:
            owners_lookup[path] = layer.owner

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

            fixed_dir = dir if dir == "/" else dir.removesuffix("/")
            owner = owners_lookup[fixed_dir]
            pkg_tar(
                name = pkg_tar_name,
                srcs = [file],
                package_dir = fixed_dir,
                mode = mode,
                owner = owner,
                tags = ["manual"],  # Exclude it from wildcard queries, e.g. "bazel build //...".
            )

    if empty_dirs:
        pkg_tar_name = name + "_pkg_tar_" + str(i)
        i += 1
        pkg_tars.append(pkg_tar_name)
        pkg_tar(
            name = pkg_tar_name,
            empty_dirs = empty_dirs.keys(),
            modes = empty_dirs,
            tags = ["manual"],  # Exclude it from wildcard queries, e.g. "bazel build //...".
        )

    if create_skia_user:
        create_home_name = name + "_create_home"
        pkg_tars.append(create_home_name)
        home(
            name = create_home_name,
            dirs = [
                dict(
                    home = "/home/" + SKIA_USERNAME,
                    uid = SKIA_UID,
                    gid = SKIA_GID,
                ),
                dict(
                    home = "/" + ROOT_USERNAME,
                    uid = ROOT_UID,
                    gid = ROOT_GID,
                ),
            ],
            tags = ["manual"],  # Exclude it from wildcard queries, e.g. "bazel build //...".
        )

        create_passwd_name = name + "_create_passwd"
        pkg_tars.append(create_passwd_name)
        passwd(
            name = create_passwd_name,
            entries = [
                dict(
                    gecos = [SKIA_USERNAME],
                    gid = SKIA_GID,
                    home = "/home/" + SKIA_USERNAME,
                    shell = "/bin/sh",
                    uid = SKIA_UID,
                    username = SKIA_USERNAME,
                ),
                dict(
                    gecos = [ROOT_USERNAME],
                    gid = ROOT_GID,
                    home = "/" + ROOT_USERNAME,
                    shell = "/bin/sh",
                    uid = ROOT_UID,
                    username = ROOT_USERNAME,
                ),
            ],
            tags = ["manual"],  # Exclude it from wildcard queries, e.g. "bazel build //...".
        )

        create_groups_name = name + "_groups"
        pkg_tars.append(create_groups_name)
        group(
            name = create_groups_name,
            entries = [
                dict(
                    name = SKIA_USERNAME,
                    gid = SKIA_GID,
                ),
            ],
            tags = ["manual"],  # Exclude it from wildcard queries, e.g. "bazel build //...".
        )

    if extra_tars:
        pkg_tars.extend(extra_tars)

    image_name = (name + "_base") if owners else name

    container_image(
        name = image_name,
        base = base_image,
        entrypoint = entrypoint,
        tars = pkg_tars,
        user = default_user,
        tags = ["manual"],  # Exclude it from wildcard queries, e.g. "bazel build //...".
        env = env,
    )

    if owners:
        for i, layer in enumerate(fixup_owners_layers):
            fixup_layer_name = name + "_fixup_owners_%d" % i
            tar_name = fixup_layer_name + "_tar"
            pkg_tar(
                name = tar_name,
                empty_dirs = layer.paths,
                owner = layer.owner,
                mode = "0755",
                tags = ["manual"],  # Exclude it from wildcard queries, e.g. "bazel build //...".
            )
            container_image(
                name = fixup_layer_name,
                base = image_name,
                entrypoint = entrypoint,
                user = default_user,
                tags = ["manual"],  # Exclude it from wildcard queries, e.g. "bazel build //...".
                tars = [tar_name],
                env = env,
            )
            image_name = fixup_layer_name

        rule_name = name
        container_image(
            name = rule_name,
            base = image_name,
            entrypoint = entrypoint,
            user = default_user,
            tags = ["manual"],  # Exclude it from wildcard queries, e.g. "bazel build //...".
            env = env,
        )
        image_name = ":" + rule_name

    container_push(
        name = "push_" + name,
        format = "Docker",
        image = image_name,
        registry = "gcr.io",
        repository = repository,
        stamp = "@io_bazel_rules_docker//stamp:always",
        tag = "{STABLE_DOCKER_TAG}",
        tags = [
            "manual",  # Exclude it from wildcard queries, e.g. "bazel build //...".
            # container_push requires the docker daemon to be
            # running. This is not possible inside RBE.
            "no-remote",
        ],
    )
