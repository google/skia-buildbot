#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compile step """

from utils import misc
from build_step import BuildStep
import shlex
import sys

class Compile(BuildStep):
  def _Run(self, args):
    make_cmd = 'make'
    if os.name == 'nt':
      make_cmd = 'make.bat'
    cmd = [make_cmd,
           args['target'],
           'BUILDTYPE=%s' % self._configuration,
           ]
    cmd += shlex.split(args['make_flags'].replace('\"', ''))
    return misc.Bash(cmd)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(Compile))