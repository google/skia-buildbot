#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This script installs the Skia Android APK.

To run:
  python /path/to/slave/scripts/install_android --device $DEVICE

"""

from utils import misc
import optparse
import sys

def main(argv):
  """ Verify that the command-line options are set and then install the APK. """
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '--device',
      help='type of device on which to install the app')
  (options, args) = option_parser.parse_args()
  if len(args) != 0:
    raise Exception('bogus command-line argument; rerun with --help')
  misc.ConfirmOptionsSet({'--device': options.device})
  misc.Install(self._serial)
  return 0

if '__main__' == __name__:
  sys.exit(main(None))
