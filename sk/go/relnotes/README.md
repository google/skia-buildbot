# The Release Notes Aggregation Library

This package will aggregate all pending release notes, which are stored in
individual files, into a single unordered list in Markdown format. This
aggregated list will be inserted at the top of the existing release notes, which
is read from a file as a new milestone heading.

The new release notes will be returned to the caller which will be responsible
for writing them to a file.