#!/bin/bash
cd testrepo
for i in `seq 1 33`;
do
  echo "Test Commit $i" > file.txt
  git add --all
  git commit -m "Test Commit $i"
  sleep 2
done
git gc --prune
cd ..
zip -r testrepo testrepo
