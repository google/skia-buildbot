#!/usr/bin/env python
#
# Copyright 2016 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Delete out directories on a Swarming bot."""


import os
import sys


workdir = '/mnt/pd0/s/c/named/work'

fail = False

for checkout in ('skia', 'src', 'pdfium'):
  out_dir = os.path.join(workdir, checkout, 'out')
  if os.path.isdir(out_dir):
    st = os.stat(out_dir)
    if st.st_uid == 0:
      print '%s is owned by root!' % out_dir
      fail = True
    else:
      print '%s not owned by root: %d' % (out_dir, st.st_uid)
  else:
    print '%s does not exist' % out_dir

if fail:
  sys.exit(1)
