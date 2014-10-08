#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Replace instances of INSERTFILE(filename) with contents of 'filename'."""


import os
import re
import sys


INSERT_FILE_PATTERN = r'INSERTFILE\((.+)\)'


def insert_file(match):
  file_to_insert = match.group(1)
  with open(os.path.expanduser(file_to_insert)) as f:
    return f.read()


def main(input_file, output_file):
  with open(input_file) as f:
    contents = f.read()
  new_contents = re.sub(INSERT_FILE_PATTERN, insert_file, contents)
  with open(output_file, 'w') as f:
    f.write(new_contents)


if __name__ == '__main__':
  if len(sys.argv) != 3:
    print >> sys.stderr, 'USAGE: %s inputfile outputfile' % sys.argv[0]
    sys.exit(1)
  main(*sys.argv[1:])

