"""
Utility script used by the oci_extract rule in //bazel:oci-extract.bzl to
extract files from an oci_image.
"""


import argparse
import json
import os
import tarfile


# Unfortunately, TarFile.extract()'s path parameter doesn't do what we'd expect
# it to do, which is to extract place the extracted file at the exact specified
# path. Instead, it treats the path as a directory into which the archive path
# of the file is nested, eg. extracting "/usr/bin/myexec" with
# path="/some/dir/myexec" produces "/some/dir/myexec/usr/bin/myexec". To work
# around this we use an extraction filter which replaces the destination path.
def make_extraction_filter(dst_path):
    def filter(tarinfo, path):
        # Call the default extraction filter first. This helps make
        # extraction more secure.
        tarinfo = tarfile.data_filter(tarinfo, path)
        tarinfo.path = dst_path
        return tarinfo
    return filter


def extract_members(input_path, members, dest_dir):
    def path_for_blob(digest):
        return os.path.join(*[input_path, "blobs"] + digest.split(":"))

    with open(os.path.join(input_path, "index.json")) as f:
        index = json.load(f)
    # We iterate in reverse, because later layers override the earlier ones and
    # we want the "final" version of the file.
    for manifest_spec in reversed(index["manifests"]):
        with open(path_for_blob(manifest_spec["digest"])) as f:
            manifest = json.load(f)
        for layer in reversed(manifest["layers"]):
            layer_tar = tarfile.open(path_for_blob(layer["digest"]))
            for src_path in list(members.keys()):
                try:
                    member = layer_tar.getmember(src_path)
                except KeyError:
                    continue
                dst_path = os.path.join(dest_dir, members[src_path])
                layer_tar.extract(member, filter=make_extraction_filter(dst_path))
                del members[src_path]

    if len(members) > 0:
        raise Exception("Failed to find and extract files from %s: %s" % (input_path, ", ".join(members.keys())))


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--input")
    parser.add_argument("--path", action="append")
    parser.add_argument("--dest", default=".")
    args = parser.parse_args()

    members = {}
    for arg in args.path:
        split = arg.split(":")
        if len(split) != 2:
            raise ValueError("Invalid path: %s" % arg)
        src = split[0].removeprefix("/")  # There's no leading "/" in the tarball.
        dst = split[1]
        members[src] = dst

    extract_members(args.input, members, args.dest)


if __name__ == "__main__":
    main()