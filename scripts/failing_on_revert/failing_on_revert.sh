#!/bin/bash

REPO=$HOME/projects/sk2/skia

go install -v ../failed_by_commit
go install -v ../odd_reverts
cd $REPO
odd_reverts > reverts.txt
cat reverts.txt | xargs -n 1 -I {}  failed_by_commit  --source_revision={} > failing_bots.txt
cat failing_bots.txt  | sed s#-All##g > failing_bots_clean.txt
cat failing_bots_clean.txt | sort | uniq --count | sort -n -r > failures_counted.txt



