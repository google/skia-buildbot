#!/usr/bin/python
# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Hooks to run after gclient sync if any files have been updated.
"""

import distutils.spawn
import os

def FindPath(name):
  """Returns the full path to the executable with this name.
  """
  return distutils.spawn.find_executable(name)


# cd to the directory where this script lives.
os.chdir(os.path.dirname(__file__))

# Work around http://code.google.com/p/chromium/issues/detail?id=89900 :
# on Windows, use already-installed gclient.bat .
internal_gclient_path = os.path.abspath(os.path.join(
    'third_party', 'depot_tools', 'gclient.bat'))
os.remove(internal_gclient_path)  # remove it now so FindPath won't find it
external_gclient_path = os.path.abspath(FindPath('gclient.bat'))
print 'replacing %s with redirect to %s' % (
    internal_gclient_path, external_gclient_path)
f = open(internal_gclient_path, 'w')
f.write('"%s" %%*' % external_gclient_path)
f.close()
