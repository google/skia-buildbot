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
        collect_packages(package_name, is_root=True)

    lockfile["packages"] = new_packages
    with open(output, "w", encoding="utf-8") as f:
        json.dump(lockfile, f, indent=2, sort_keys=True)
        f.write("\n")


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