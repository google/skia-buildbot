import os
import sys

path = "/home/chrome-bot/.bazelrc"
if os.path.exists(path):
    print(f"BAZELRC_EXISTS: TRUE")
    print(f"Contents of {path}:")
    try:
        with open(path, "r") as f:
            print(f.read())
    except Exception as e:
        print(f"Error reading {path}: {e}")
else:
    print(f"BAZELRC_EXISTS: FALSE")
