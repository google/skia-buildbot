#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This script runs a Skia binary inside of an Android APK.

To run:
  python /path/to/slave/scripts/run_android --binary_name $BINARY --args $ARGS

Where BINARY is the name of the Skia binary to run, eg. gm or tests
And ARGS are any command line arguments to pass to that binary.

For example:
  python /path/to/slave/scripts/run_android --binary_name gm --args --nopdf

"""

import optparse
import skia_slave_utils
import sys

def main(argv):
  """ Verify that the command-line options are set and then run the APK. """
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '--binary_name',
      help='name of the Skia binary to launch')
  option_parser.add_option(
      '--device',
      help='type of device on which to run the binary')
  option_parser.add_option(
      '--args',
      help='arguments to pass to the Skia binary')
  (options, args) = option_parser.parse_args()
  if len(args) != 0:
    raise Exception('bogus command-line argument; rerun with --help')
  skia_slave_utils.ConfirmOptionsSet({
      '--binary_name': options.binary_name,
      '--device': options.device,
      })
  serial = skia_slave_utils.GetSerial(options.device)
  skia_slave_utils.Run(serial, options.binary_name, arguments=options.args)
  return 0

if '__main__' == __name__:
  sys.exit(main(None))
