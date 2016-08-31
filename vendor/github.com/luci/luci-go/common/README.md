luci-go: LUCI in Go: shared code
================================

[![GoDoc](https://godoc.org/github.com/luci/luci-go/common?status.svg)](https://godoc.org/github.com/luci/luci-go/common)

Strive to keep this directory and subdirectories from having more than 7-10
libraries. If they grow too large, consider grouping them into informative
subcategories.

The following sub-directories exist:

  * data - **data organization, manipulation, storage**
  * sync - **concurrency coordination**
  * runtime - **libraries relating to debugging/enhancing the go runtime**
  * system - **libraries for interacting with the operating system**
