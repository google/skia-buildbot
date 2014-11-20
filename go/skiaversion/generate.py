#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Generate Go source with version information."""


import subprocess
import sys

def generate_version_file(source_file, output_file):
  version_info = {
    'commit_hash':
        subprocess.check_output(['git', 'rev-parse', 'HEAD']).rstrip(),
    'date': subprocess.check_output(['date', '--rfc-3339=seconds']).rstrip(),
  }
  with open(output_file, 'w') as o:
    with open(source_file) as i:
      o.write(i.read() % version_info)


if __name__ == '__main__':
  generate_version_file(*sys.argv[1:])
