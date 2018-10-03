#/bin/bash

# Creates the tables for Perf in BigTable.

# Skia instance.
cbt --instance perf-bt createtable skia families=V:maxversions=1,S:maxversions=1,D:maxversions=1,H:maxversions=1
