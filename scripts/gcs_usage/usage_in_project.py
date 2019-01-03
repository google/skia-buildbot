#!/usr/bin/env python
#
# Copyright 2019 Google LLC
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import argparse
import subprocess

def main():
  parser = argparse.ArgumentParser()
  parser.add_argument(
      '--project', help='The GCP project to analyze usage for.', required=True)
  parser.add_argument(
      '--bytes', help='Show the results in bytes, not in human readable form.',
                 action='store_true')
  args = parser.parse_args()

  print 'Fetching buckets in project'
  buckets = subprocess.check_output(['gsutil', 'ls', '-p', args.project])

  buckets = buckets.split()

  print 'Found %d buckets' % len(buckets)
  print 'Tabulating total space, this may take tens of seconds for big buckets'

  flags = '-hs'
  if args.bytes:
    flags = '-s'

  # Do them one at a time to show incremental progress, as large
  # buckets can take >10s to tabulate.
  for b in buckets:
    # GCS buckets must be uniquely named, so no need to specify a project.
    print subprocess.check_output(['gsutil', 'du', flags, b]).strip()

  print 'Done!'

if __name__ == '__main__':
  main()
