#!/usr/bin/env python
# Copyright (c) 2015 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Runs make test."""

import os
import subprocess
import sys

CWD = os.path.dirname(os.path.abspath(__file__))

TEST_FAILED = (
'''======================================================================
make test failed; CWD: %s
----------------------------------------------------------------------
%s
----------------------------------------------------------------------
''')

def RunMakeTest(cwd):
  p = subprocess.Popen(['make', 'test'], cwd=cwd,
                       stderr=subprocess.STDOUT,
                       stdout=subprocess.PIPE)
  if p.wait() != 0:
    return [TEST_FAILED % (cwd, p.communicate()[0])]
  return []

if __name__ == '__main__':
  errors = RunMakeTest(CWD);
  if errors:
    for error in errors:
      print error
    sys.exit(1)
