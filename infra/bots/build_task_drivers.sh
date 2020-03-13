#!/bin/bash

# Takes a single argument that is the output directory where executables are to
# be placed.

set -x -e

if [ -z "${1}" ]; then
  echo "Usage: ${0} <output-dir>"
  exit 1
fi

echo "Writing task drivers to ${1}"
mkdir -p ${1}

task_drivers_abs="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )/task_drivers"
task_drivers_dir="./$(realpath --relative-to=$(pwd) ${task_drivers_abs})"
for td in $(cd ${task_drivers_dir} && ls); do
  go build -o ${1}/${td} ${task_drivers_dir}/${td}
done
