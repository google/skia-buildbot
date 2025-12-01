"""
Utility script used by //bazel:node_modules.bzl to strip package-lock.json down
to only the dependencies of a specific package.
"""


import argparse
import json
import os


def strip_package_json(input, output, package_name, include_dev=False,
                       include_optional=False):
    with open(input, "r", encoding="utf-8") as f:
        package_json = json.load(f)

    old_deps = package_json["dependencies"]
    new_deps = {
        package_name: old_deps[package_name]
    }
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


    def collect_packages(package_name, parent_key=None):
        # Find the package in the "packages" dictionary.
        key = find_package_key(package_name, parent_key=parent_key)
        if key is None:  # An empty string is valid for key.
            raise Exception("%s not in spec", package_name)
        if new_packages.get(key):
            return

        # Copy the package information.
        package_spec = old_packages[key]
        if parent_key is None and package_spec.get("bin"):
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

    collect_packages(package_name)

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

    strip_package_lock(os.path.join(args.input, "package-lock.json"),
                       os.path.join(args.output, "package-lock.json"),
                       args.package,
                       args.include_dev,
                       args.include_optional)
    strip_package_json(os.path.join(args.input, "package.json"),
                       os.path.join(args.output, "package.json"),
                       args.package,
                       args.include_dev,
                       args.include_optional)


if __name__ == "__main__":
    main()