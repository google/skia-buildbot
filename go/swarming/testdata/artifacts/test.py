# Copyright 2016 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import optparse
import os
import sys


parser = optparse.OptionParser()
parser.add_option("", "--arg1", dest="arg1")
parser.add_option("", "--arg2", dest="arg2")
parser.add_option("", "--output-dir", dest="output_dir")

options, _ = parser.parse_args()

if not options.arg1 or not options.arg2 or not options.output_dir:
  sys.exit(1)

print options.arg1
print options.arg2

f = open(os.path.join(options.output_dir, 'output.txt'), 'w')
f.write('testing\ntesting')

sys.exit(0)