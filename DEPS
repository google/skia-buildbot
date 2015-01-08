use_relative_paths = True

vars = {
  "skia_git": "https://skia.googlesource.com",
}

deps = {
  # Utilities shared between the Skia and Skia-Buildbot repositories.
  "common":
    Var("skia_git") + "/common.git@6683b15b039a31d5d86ce6c8af4dd56861d10ee4",
}

recursedeps = [
  "common",
]
