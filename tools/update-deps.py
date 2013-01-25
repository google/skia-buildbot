#!/usr/bin/python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run this script to update chromium_revision within the buildbot DEPS file.
"""

import os
import re
import subprocess


def GetSvnRevision(file_or_url):
  """Returns the revision of this file within its Subversion repository
     (or the latest revision of a given Subversion repository).
     Raises IOError if unable to find the revision of this file/URL.
  """
  command = ['svn', 'info', file_or_url]
  popen = subprocess.Popen(command, stdout=subprocess.PIPE, shell=False)
  output = popen.stdout.read()
  matches = re.findall('Revision: (\d+)$', output, re.MULTILINE)
  if not matches:
    raise IOError('could not find revision of file_or_url %s' % file_or_url)
  return matches[0]


def ReplaceDepsVar(deps_file, variable_name, value):
  """Replaces the definition of a variable within this DEPS file, and writes
     the new version of the file out in place of the old one.
  """
  with open(deps_file, 'r') as file_handle:
    contents_old = file_handle.read()
  contents_new = re.sub(
      '"%s":.*,' % variable_name,
      '"%s": "%s",' % (variable_name, value),
      contents_old)
  with open(deps_file, 'w') as file_handle:
    file_handle.write(contents_new)


def Main():
  # cd to the root directory of this checkout.
  os.chdir(os.path.join(os.path.dirname(__file__), os.path.pardir))

  # Update chromium_revision in standard DEPS file.
  chromium_rev = GetSvnRevision('http://src.chromium.org/svn/trunk')
  ReplaceDepsVar('DEPS', 'chromium_revision', chromium_rev)


if __name__ == '__main__':
  Main()
