#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility to detect invalid SKPs and remove from local dir."""

import glob
import optparse
import os
import sys

import shell_utils


def IsSKPValid(path_to_skp, path_to_skpinfo):
  """Calls the skpinfo binary to see if the specified SKP is valid."""
  skp_info_cmd = [path_to_skpinfo, '-i', path_to_skp]
  try:
    shell_utils.run(skp_info_cmd)
    return True
  except shell_utils.CommandFailedException:
    # Mark SKP as invalid if the skpinfo command gives a non 0 ret code.
    return False


def RemoveInvalidSKPs(path_to_skp, path_to_skpinfo):
  """Removes invalid SKPs from the provided local dir."""
  if not IsSKPValid(path_to_skp, path_to_skpinfo):
    print '=====%s is invalid!=====' % path_to_skp
    os.remove(path_to_skp)
    print '=====Deleted the SKP=====\n\n'


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
    '', '--path_to_skp', help='Path to the SKP we want to check.')
  option_parser.add_option(
    '', '--path_to_skpinfo', help='Complete path to the skpinfo binary.')

  options, unused_args = option_parser.parse_args()
  if not (options.path_to_skp and options.path_to_skpinfo):
    option_parser.error('Must specify path_to_skp and path_to_skpinfo')

  RemoveInvalidSKPs(path_to_skp=options.path_to_skp,
                    path_to_skpinfo=options.path_to_skpinfo)

