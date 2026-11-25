"""
Utility script used by //bazel:node_modules.bzl to strip package-lock.json down
to only the dependencies of a specific package.
"""


import argparse
import json
import os


def strip_package_json(input, output, package_name, include_dev=False,
                       include_optional=False, lockfile_input=None):
    with open(input, "r", encoding="utf-8") as f:
        package_json = json.load(f)

    old_deps = package_json["dependencies"]
    if package_name in old_deps:
        new_deps = {
            package_name: old_deps[package_name]
        }
    elif lockfile_input:
        with open(lockfile_input, "r", encoding="utf-8") as lf:
            lockfile = json.load(lf)
        short_name = package_name.split("node_modules/")[-1]
        new_deps = {
            short_name: lockfile["packages"]["node_modules/" + package_name]["version"]
        }
    else:
        raise KeyError(package_name)
    package_json["dependencies"] = new_deps
    if not include_dev and package_json.get("devDependencies"):
        del package_json["devDependencies"]
    if not include_optional and package_json.get("optionalDependencies"):
        del package_json["optionalDependencies"]

    with open(output, "w", encoding="utf-8") as f:
        json.dump(package_json, f, indent=2, sort_keys=True)
        f.write("\n")


def strip_package_lock(input, output, package_name,
                       include_dev=False, include_optional=False):
    with open(input, "r", encoding="utf-8") as f:
        lockfile = json.load(f)

    old_packages = lockfile["packages"]
    new_packages = {}

    def find_package_key(package_name, components=None, parent_key=None):
        """Find the location of the package in the dictionary.

        Some shared dependencies are hoisted upwards in the directory structure,
        but others remain in the node_modules subdirectory of the package which
        depends on them.
        """
        if package_name == "":
            return package_name

        if not components:
            components = []
            if parent_key:
                components.extend(parent_key.split("/"))
            components.append("node_modules")

        key = "/".join(components + [package_name])
        if old_packages.get(key):
            return key

        # Pop components until we find another "node_modules" directory.
        components.pop()
        while components and components[-1] != "node_modules":
            components.pop()

        return find_package_key(package_name, components=components)


    def collect_packages(package_name, parent_key=None, is_root=False):
        # Find the package in the "packages" dictionary.
        key = find_package_key(package_name, parent_key=parent_key)
        if key is None:  # An empty string is valid for key.
            raise Exception("%s not in spec", package_name)
        if new_packages.get(key):
            return

        # Copy the package information.
        package_spec = dict(old_packages[key])
        package_spec.pop("dev", None)
        if (parent_key is None or is_root) and package_spec.get("bin"):
            # This results in a symlink that becomes broken after moving the
            # pacakge directory.
            del package_spec["bin"]
        new_packages[key] = package_spec

        # Recurse on any dependencies.
        for dep in package_spec.get("dependencies", []):
            collect_packages(dep, parent_key=key)
        if include_dev:
            for dep in package_spec.get("devDependencies", []):
                collect_packages(dep, parent_key=key)
        if include_optional:
            for dep in package_spec.get("optionalDependencies", []):
                collect_packages(dep, parent_key=key)

    if "/node_modules/" in package_name:
        parent_prefix, short_name = package_name.rsplit("/node_modules/", 1)
        parent_key = "node_modules/" + parent_prefix
        collect_packages(short_name, parent_key=parent_key, is_root=True)
        hoisted_packages = {}
        prefix_to_strip = parent_key + "/"
        for k, v in new_packages.items():
            if k.startswith(prefix_to_strip):
                hoisted_packages[k[len(prefix_to_strip):]] = v
            else:
                hoisted_packages[k] = v
        new_packages = hoisted_packages
    else:
        short_name = package_name
        collect_packages(package_name, is_root=True)

    new_packages = promote_target_nested_deps(new_packages, short_name)

    lockfile["packages"] = new_packages
    with open(output, "w", encoding="utf-8") as f:
        json.dump(lockfile, f, indent=2, sort_keys=True)
        f.write("\n")


def _extract_pkg_name(rel_path):
    """Return the package name (including @scope if scoped) from a path under node_modules/."""
    parts = rel_path.split("/")
    if parts[0].startswith("@") and len(parts) >= 2:
        return parts[0] + "/" + parts[1]
    return parts[0]


def _resolves_to_top_level(packages, pkg_key, dep_name):
    """Return True if pkg_key resolves dep_name to node_modules/<dep_name>."""
    spec = packages[pkg_key]
    all_deps = (
        set(spec.get("dependencies", ()))
        | set(spec.get("optionalDependencies", ()))
        | set(spec.get("peerDependencies", ()))
    )
    if dep_name not in all_deps:
        return False

    # Walk up from pkg_key to see if a closer nested copy shadows the top-level one.
    parts = pkg_key.split("/")
    while parts:
        candidate = "/".join(parts) + "/node_modules/" + dep_name
        if candidate in packages:
            return False
        parts.pop()
        while parts and parts[-1] != "node_modules":
            parts.pop()
        if parts and parts[-1] == "node_modules":
            parts.pop()
    return True


def promote_target_nested_deps(packages, short_name):
    """Promote node_modules/<short_name>/node_modules/* to top-level node_modules/*.

    node_module_tar hoists the target package directory up one level after
    `npm ci`. If the target package has nested dependencies under
    node_modules/<short_name>/node_modules/, copying them over top-level
    node_modules/ would silently merge incompatible versions. Promoting them in
    the lockfile first—and relocating any displaced top-level package under the
    dependents that require it—lets `npm ci` build a clean, collision-free tree.
    """
    root_key = "node_modules/" + short_name
    nested_prefix = root_key + "/node_modules/"

    direct_nested = sorted({
        _extract_pkg_name(k[len(nested_prefix):])
        for k in packages
        if k.startswith(nested_prefix)
    })
    if not direct_nested:
        return packages

    # Snapshot any top-level packages that will be displaced by the promotion.
    displaced = {}
    for dep_name in direct_nested:
        top_key = "node_modules/" + dep_name
        if top_key in packages:
            sub_prefix = top_key + "/node_modules/"
            displaced[dep_name] = {
                k: dict(v)
                for k, v in packages.items()
                if k == top_key or k.startswith(sub_prefix)
            }

    # Resolve chained displacements to a fixpoint: if a package inside
    # displaced[outer] resolved another displaced package `inner` at top level,
    # nest displaced[inner] inside displaced[outer] so every relocated copy of
    # `outer` carries the right `inner`.
    changed = True
    while changed:
        changed = False
        for outer_name, outer_subtree in displaced.items():
            for pk in list(outer_subtree):
                for inner_name, inner_subtree in displaced.items():
                    if inner_name == outer_name:
                        continue
                    if not _resolves_to_top_level(outer_subtree, pk, inner_name):
                        continue
                    target_key = pk + "/node_modules/" + inner_name
                    if target_key in outer_subtree:
                        continue
                    # Guard against mutual dependency cycles among displaced packages.
                    if ("node_modules/" + inner_name + "/") in (pk + "/"):
                        continue
                    inner_top = "node_modules/" + inner_name
                    for sub_k, sub_v in inner_subtree.items():
                        suffix = sub_k[len(inner_top):]
                        new_k = target_key + suffix
                        if new_k not in outer_subtree:
                            outer_subtree[new_k] = dict(sub_v)
                            changed = True

    displaced_keys = {k for sub in displaced.values() for k in sub}

    # Find every non-displaced package outside root_key that resolves to a
    # displaced top-level package.
    relocations = {}
    for dep_name in displaced:
        dependents = []
        for pk in packages:
            if pk == root_key or pk.startswith(nested_prefix):
                continue
            if pk in displaced_keys:
                continue
            if _resolves_to_top_level(packages, pk, dep_name):
                dependents.append(pk)
        relocations[dep_name] = dependents

    result = {}
    for k, v in packages.items():
        if k in displaced_keys:
            continue
        if k.startswith(nested_prefix):
            result["node_modules/" + k[len(nested_prefix):]] = v
        else:
            result[k] = v

    # Re-insert displaced packages (with any chained displacements already
    # nested inside them) under each dependent that resolved to them.
    for dep_name, dependents in relocations.items():
        top_key = "node_modules/" + dep_name
        subtree = displaced[dep_name]
        for pk in dependents:
            for sub_k, sub_v in subtree.items():
                suffix = sub_k[len(top_key):]
                new_k = pk + "/node_modules/" + dep_name + suffix
                result[new_k] = dict(sub_v)

    assert not any(k.startswith(nested_prefix) for k in result), (
        "unexpected nested packages remaining under %s" % nested_prefix
    )
    return result


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("-i", "--input", default=".")
    parser.add_argument("-o", "--output")
    parser.add_argument("-p", "--package")
    parser.add_argument("-d", "--include-dev", action="store_true")
    parser.add_argument("--include-optional", action="store_true")
    args = parser.parse_args()

    lockfile_in = os.path.join(args.input, "package-lock.json")
    strip_package_lock(lockfile_in,
                       os.path.join(args.output, "package-lock.json"),
                       args.package,
                       args.include_dev,
                       args.include_optional)
    strip_package_json(os.path.join(args.input, "package.json"),
                       os.path.join(args.output, "package.json"),
                       args.package,
                       args.include_dev,
                       args.include_optional,
                       lockfile_input=lockfile_in)


if __name__ == "__main__":
    main()