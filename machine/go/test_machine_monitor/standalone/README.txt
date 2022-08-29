Package standalone gets hardware information about Skolo machines that do not delegate test-running
to an attached or SSH-accessed device.

A Raspberry Pi with no device associated is also technically "standalone". This saves some
complexity. Otherwise, we would have needed to introduce a fourth state ("unattached" or similar) in
the iOS/Android/SSH/standalone enumeration for RPis, which would have opened up the possibility of
mis-targeting tasks by mis-clicking in the machineserver GUI.

A lot of code in this package is platform-specific, so there is one file per platform, and Go's
build-constraint magic examines the file names to pick the right one. See
https://pkg.go.dev/cmd/go#hdr-Build_constraints.
