import os
import sys

path = "/home/chrome-bot/.bazelrc"
if os.path.exists(path):
    print(f"BAZELRC_EXISTS: TRUE")
    print(f"Removing {path}...")
    try:
        os.remove(path)
        print(f"Successfully removed {path}.")
    except Exception as e:
        print(f"Error removing {path}: {e}")
        sys.exit(1)
else:
    print(f"BAZELRC_EXISTS: FALSE. No removal needed.")
