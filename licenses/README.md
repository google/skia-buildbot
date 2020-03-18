# License Tracking

The LICENSES.md file contains a list of all known recursive dependencies of all
code in this repo and their licenses.

To check the whether the dependencies have changed and whether the licenses are
compatible, run

    make

in this directory.

## Failures

The test may fail because the dependencies have changed, or because a dependency
has a license which is incompatible with ours. In the former case, run

    make regenerate

to write the updated list of packages to LICENSES.md. If a dependency has an
incompatible license, it needs to be removed.

## Manual Checks

The package github.com/daaku/go.zipexe lists as unknown, but is MIT licensed.
