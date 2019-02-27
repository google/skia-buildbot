License Tracking
================

The file 'all_deps.txt' contains a list of all known recursive
dependencies of all code in this repo. For each of those dependencies
we have found and recorded their license in LICENSES.md.

To check if any new dependencies have appeared, run

    make

in this directory.

Failures
--------

If the test fails it will print all the new packages. For each new package
add a new line to LICENSES.md and then run:

    make regenerate

to write the updated list of packages to 'all_deps.txt'.

Note that the check will also fail if a dependency is removed,
in which case remove the associated package(s) from LICENSES.md
and then run:

     make regenerate

