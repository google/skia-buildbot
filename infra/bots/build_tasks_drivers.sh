#!/bin/bash

set -x -e

go build -o ${1} ./test_drivers/*
