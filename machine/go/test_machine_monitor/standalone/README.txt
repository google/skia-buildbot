Package standalone gets hardware information about Skolo machines that run tests themselves
rather than on attached devices.

A lot of this is platform-specific, so there is one file per platform, and Go's build-constraint
magic examines the file names to pick the right one. See
https://pkg.go.dev/cmd/go#hdr-Build_constraints.
