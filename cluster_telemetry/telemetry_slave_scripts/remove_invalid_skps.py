#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to detect invalid SKPs and remove from local dir."""

import glob
import optparse
import os
import sys

# Set the PYTHONPATH for this script to include shell_utils.
sys.path.append(
    os.path.join(os.path.dirname(os.path.realpath(__file__)), os.pardir,
                 os.pardir, os.pardir, 'slave', 'skia_slave_scripts'))
from utils import shell_utils


def IsSKPValid(path_to_skp, path_to_skpinfo):
  """Calls the skpinfo binary to see if the specified SKP is valid."""
  skp_info_cmd = [path_to_skpinfo, '-i', path_to_skp]
  try:
    shell_utils.run(skp_info_cmd)
    return True
  except shell_utils.CommandFailedException:
    # Mark SKP as invalid if the skpinfo command gives a non 0 ret code.
    return False


def RemoveInvalidSKPs(skp_dir, path_to_skpinfo):
  """Removes invalid SKPs from the provided local dir."""
  invalid_skps = []
  for path_to_skp in glob.glob(os.path.join(skp_dir, '*.skp')):
    if not IsSKPValid(path_to_skp, path_to_skpinfo):
      print '=====%s is invalid!=====' % path_to_skp
      invalid_skps.append(path_to_skp)
      # Delete the SKP from the local path.
      os.remove(path_to_skp)

  if invalid_skps:
    print '\n\n=====Deleted the following SKPs:====='
    for invalid_skp in invalid_skps:
      print invalid_skp


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
    '', '--skp_dir', help='Directory that contains the SKPs we want to check.')
  option_parser.add_option(
    '', '--path_to_skpinfo', help='Complete path to the skpinfo binary.')

  options, unused_args = option_parser.parse_args()
  if not (options.skp_dir and options.path_to_skpinfo):
    option_parser.error('Music specify skp_dir and path_to_skpinfo')

  RemoveInvalidSKPs(skp_dir=options.skp_dir,
                    path_to_skpinfo=options.path_to_skpinfo)

