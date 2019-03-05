#/bin/bash

# Creates the tables for Perf in BigTable.

# Skia instance.
cbt --instance production createtable perf-skia families=V:maxversions=1,S:maxversions=1,D:maxversions=1,H:maxversions=1
cbt --instance perf-bt createtable android families=V:maxversions=1,S:maxversions=1,D:maxversions=1,H:maxversions=1
cbt --instance perf-bt createtable ct families=V:maxversions=1,S:maxversions=1,D:maxversions=1,H:maxversions=1
