"""This module has some helpers for generating list of files."""

def config_files(server, instances):
    """Generate a list of config files for every passed in instance.

    Args:
      server: string corresponding to the server name
      instances: list of strings corresponding to the instances that have
        this file.

    Returns:
      A list of strings (files) that can configure "server" for all instances.
    """
    files = []
    for instance in instances:
        files += [
            "//golden/k8s-instances:" + instance + "/" + instance + ".json5",
            "//golden/k8s-instances:" + instance + "/" + instance + "-spanner.json5",
            "//golden/k8s-instances:" + instance + "/" + instance + "-" + server + ".json5",
        ]
    return files
