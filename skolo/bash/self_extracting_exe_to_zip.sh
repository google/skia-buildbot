#!/bin/sh -e

# This script will convert a Windows self-extracting installer to a ZIP archive.
#
# Requires the Z-zip application:
#
#    apt install 7zip

if [ $# -eq 0 ]; then
  >&2 echo "Must provide driver setup executable file"
  exit 1
fi

PWD=$(pwd)

EXE_FILE=$1
EXE_DIR=$(dirname -- "${EXE_FILE}")

BASENAME=${EXE_FILE%.*}
ABS_DIR=$(realpath "${EXE_DIR}")
ZIP_FILE=${ABS_DIR}/${BASENAME}.zip
TEMP_DIR=/tmp/temp_exe_extract_dir

rm -rf ${TEMP_DIR}
rm -f ${ZIP_FILE}

mkdir ${TEMP_DIR}
7zz x ${EXE_FILE} -o${TEMP_DIR}
cd ${TEMP_DIR}
7zz a ${ZIP_FILE} *
cd ${PWD}
rm -rf ${TEMP_DIR}
