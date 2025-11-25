"""
This file contains a helper function used to derive layers in a container
based on specified directory ownership.

There is a quirk in the way that pkg_tar interacts with container_image which
can result in problems with directory ownership. The skia_app_container macro
creates a separate pkg_tar for each file added to the image. Because the
resulting tar files have no knowledge of the directory layout and ownership of
the previous layer, each tar file will build a path to the specified file, with
all of the containing directories having the same owner as the one specified for
the file. The tar files are merged to become a layer on top of the previous
layer, which causes any existing directories from the previous layer to be
overwritten by the directories from the tar files, including their ownership.
This creates complications when we add files with a particular owner (the
default being root) to directories with other owners (eg. /home/skia), causing
the user's home directory to be owned by root.

Our solution to this is to create a fixup layers after the pkg_tar layer which
overlay the correct ownership onto each directory. In order to do this, we
need the caller to specify which directories should be owned by which user,
and then we build a tree containing all of the paths that were given,
propagating the correct ownership to each directory. Then, we find the subtrees
which have the same owner, sort them by depth, and create layers from them to
ensure that each set of directories has the correct owner.
"""

# TODO(borenet): This was a convenient place to put these constants due to the
# import dependency graph, but it would make more logical sense for them to be
# in a centralized "settings for containers" file.
ROOT_UID = 0
ROOT_GID = 0
ROOT_UID_GID = "%d.%d" % (ROOT_UID, ROOT_GID)
ROOT_USERNAME = "root"

SKIA_UID = 2000
SKIA_GID = 2000
SKIA_UID_GID = "%d.%d" % (SKIA_UID, SKIA_GID)
SKIA_USERNAME = "skia"

def _node(name, owner, children, depth):
    return struct(
        name = name,
        owner = owner,
        children = children,
        depth = depth,
    )

def _layer(owner, paths):
    return struct(
        owner = owner,
        paths = paths,
    )

def _add_child(root, path, owners):
    # Starlark doesn't support recursion, which would make this a lot simpler...
    owner = "0.0"
    depth = 0
    parent = root
    for _ in range(99999999):  # Starlark doesn't support while-statements...
        split = path.split("/")
        key = split[0]
        path = "/".join(split[1:])
        child_path = parent.name.removesuffix("/") + "/" + key
        owner = owners.get(child_path, owner)
        if key not in parent.children:
            parent.children[key] = _node(name = child_path, owner = owner, children = {}, depth = depth + 1)
        parent = parent.children[key]
        depth += 1
        if not path:
            break

def _subtrees_by_owner(root):
    # Starlark doesn't support recursion, which would make this a lot simpler...
    stack = [root]
    returns = {}
    for _ in range(99999999):  # Starlark doesn't support while-statements...
        node = stack[-1]
        done = True
        for child in reversed(node.children.values()):
            if not returns.get(child.name):
                stack.append(child)
                done = False
        if done:
            this_node = _node(node.name, node.owner, children = {}, depth = node.depth)
            subtrees = [this_node]
            for key, child in node.children.items():
                child_subtrees = returns[child.name]
                if child.owner == node.owner:
                    # The first subtree is rooted at the node itself.
                    this_node.children[key] = child_subtrees[0]
                    child_subtrees = child_subtrees[1:]
                subtrees.extend(child_subtrees)
            returns[node.name] = subtrees
            stack.pop()
        if not stack:
            break
    return returns[root.name]

def _get_paths(root):
    # Starlark doesn't support recursion, which would make this a lot simpler...
    stack = [root]
    paths = []
    for _ in range(99999999):  # Starlark doesn't support while-statements...
        node = stack.pop()
        paths.append(node.name)
        stack.extend(node.children.values())
        if not stack:
            break
    return paths

def _build_owners_tree(dirs, owners):
    # Let root be an extra level above "/" and just remove it later. This
    # bypasses a lot of fiddling in the recursive code.
    root = _node(name = "", owner = "0.0", children = {}, depth = 0)
    for path in dirs + list(owners.keys()):
        _add_child(root, path, owners)
    return root.children[""]

def get_fixup_owners_layers(dirs, owners = None):
    """Derive a set of layers to fix up directory owners in an image.

    Args:
        dirs: list of directories, eg. ["/usr/local/bin", "/home/skia/my-dir"]
        owners: a dict with directory paths as keys and owners in the form of
            "uid.gid" as values, eg. {"/home/skia": "2000.2000"}

    Returns:
        A list of layer instances.
    """
    tree = _build_owners_tree(dirs, owners = owners)
    subtrees = _subtrees_by_owner(tree)
    subtrees = sorted(subtrees, key = lambda node: node.depth, reverse = True)
    layers = []
    for subtree in subtrees:
        layers.append(_layer(
            owner = subtree.owner,
            paths = sorted(_get_paths(subtree)),
        ))
    return layers
