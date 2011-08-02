#!/usr/bin/python
# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""DEPS-developer will run this after gclient sync if any files have been
   updated.
"""

import os
import re
import subprocess

# local
import hooks

def GetSvnRevision(file):
  """Returns the revision of this file within its Subversion repository.
     Raises IOError if unable to find the revision of this file.
  """
  command = ['svn', 'info', file]
  popen = subprocess.Popen(command, stdout=subprocess.PIPE, shell=False)
  output = popen.stdout.read()
  matches = re.findall('Revision: (\d+)$', output, re.MULTILINE)
  if not matches:
    raise IOError('could not find revision of file %s' % file)
  return matches[0]

def ReplaceDepsVar(file, variable_name, value):
  """Replaces the definition of a variable within this DEPS file, and writes
     the new version of the file out in place of the old one.
  """
  with open(file, 'r') as file_handle:
    contents_old = file_handle.read()
  contents_new = re.sub(
      '"%s":.*,' % variable_name,
      '"%s": "%s",' % (variable_name, value),
      contents_old)
  with open(file, 'w') as file_handle:
    file_handle.write(contents_new)

def Main():
  # cd to the directory where this script lives.
  os.chdir(os.path.dirname(__file__))

  # Update chromium_revision in standard DEPS file.
  chromium_rev = GetSvnRevision('third_party/chromium_buildbot/DEPS')
  ReplaceDepsVar('DEPS', 'chromium_revision', chromium_rev)

  # Chain to the standard hooks (as run by the standard DEPS file).
  hooks.Main()

if __name__ == '__main__':
  Main()
