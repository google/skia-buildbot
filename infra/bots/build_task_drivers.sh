#!/bin/bash

# Takes a single argument that is the output directory where executables are to
# be placed.

set -x -e

if [ -z "${1}" ]; then
  echo "Usage: ${0} <output-dir>"
  exit 1
fi

env

out=$(realpath $1)
echo "Writing task drivers to ${out}"
mkdir -p ${out}

script_dir=$(dirname ${BASH_SOURCE[0]})
cd "${script_dir}"
task_drivers_dir="./task_drivers"
for td in $(cd ${task_drivers_dir} && ls); do
  go build -o ${out}/${td} ${task_drivers_dir}/${td}
done
