#!/usr/bin/env python
# Copyright (c) 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run the Go tests and report any uncategorized tests. Exit 1 if any."""


# TODO(borenet): Delete this once we've switched completely to running tests
# within Bazel and no longer need to categorize tests.


from __future__ import print_function
import re
import subprocess
import sys


REGEX = '^--- ([A-Z]+): (\w+)'
SKIP = 'SKIP'


def main():
  cmd = ['go', 'test', '-v', './...', '--uncategorized']
  try:
    output = subprocess.check_output(cmd)
  except subprocess.CalledProcessError as e:
    output = e.output
  notskipped = []
  for line in output.splitlines():
    m = re.search(REGEX, line)
    if m:
      result, name = m.groups()
      if result != SKIP:
        notskipped.append(name)

  if notskipped:
    print('%d tests are not categorized:' % len(notskipped))
    for t in notskipped:
      print('\t%s' % t)
    sys.exit(1)


if __name__ == '__main__':
  main()
