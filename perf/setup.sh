#/bin/bash

# Creates the tables for Perf in BigTable.

# Skia instance.
cbt --instance production createtable perf-skia families=V:maxversions=1,S:maxversions=1,D:maxversions=1,H:maxversions=1,I:maxversions=1
cbt --instance production createtable perf-android families=V:maxversions=1,S:maxversions=1,D:maxversions=1,H:maxversions=1,I:maxversions=1
cbt --instance production createtable perf-ct families=V:maxversions=1,S:maxversions=1,D:maxversions=1,H:maxversions=1,I:maxversions=1
