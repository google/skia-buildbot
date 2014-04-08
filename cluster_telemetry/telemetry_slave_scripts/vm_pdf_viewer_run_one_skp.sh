#!/bin/bash

FULL_PATH_SKP="$2"
LOGS_DIR="$1"
SKP=`basename "$2" .skp`

out/Release/render_pictures -r $FULL_PATH_SKP -w $LOGS_DIR/expected/ >> $LOGS_DIR/logs/$SKP.txt
out/Release/render_pdfs $FULL_PATH_SKP -w $LOGS_DIR/pdf/ >> $LOGS_DIR/logs/$SKP.txt
out/Release/pdfviewer -r $LOGS_DIR/pdf/$SKP.pdf -w $LOGS_DIR/actual/ -n >> $LOGS_DIR/logs/$SKP.txt
out/Release/chop_transparency -r $LOGS_DIR/expected/$SKP.png
out/Release/chop_transparency -r $LOGS_DIR/actual/$SKP.png
out/Release/skpdiff -p $LOGS_DIR/expected/$SKP.png $LOGS_DIR/actual/$SKP.png --csv $LOGS_DIR/csv/$SKP.csv >> $LOGS_DIR/logs/$SKP.txt
