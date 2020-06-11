#!/bin/bash

# Sets up a port-forward to the CockroachDB instance.

kubectl port-forward perf-android-cockroachdb-0 26257
