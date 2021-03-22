#!/usr/bin/env python
#
# Copyright 2016 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Delete out directories on a Swarming bot."""


from __future__ import print_function
import os
import shutil


workdir = '/b/work'


for checkout in ('skia', 'src', 'pdfium'):
  out_dir = os.path.join(workdir, checkout, 'out')
  if os.path.isdir(out_dir):
    print('Deleting %s' % out_dir)
    shutil.rmtree(out_dir)
  else:
    print('Unable to find checkout out dir: %s' % out_dir)
