#!/usr/bin/python
# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Hooks to run after gclient sync if any files have been updated.
"""

import os

def FindPath(name):
  """Returns the full path to the executable with this name, or None if not
  found
  """
  for trydir in os.environ["PATH"].split(os.pathsep):
    trypath = os.path.join(trydir, name)
    if os.path.exists(trypath):
      return trypath
  return None


# cd to the directory where this script lives.
os.chdir(os.path.dirname(__file__))

# Work around http://code.google.com/p/chromium/issues/detail?id=89900 :
# on Windows, redirect local gclient.bat to already-installed gclient.bat .
batchfile = 'gclient.bat'
internal_gclient_path = os.path.abspath(os.path.join(
    'third_party', 'depot_tools', batchfile))
external_gclient_path = FindPath(batchfile)
if not external_gclient_path:
  raise OSError('could not find external version of %s' % batchfile)
elif internal_gclient_path == external_gclient_path:
  print ('internal_gclient_path == external_gclient_path == "%s"' %
         external_gclient_path)
else:
  os.remove(internal_gclient_path)
  print 'replacing %s with redirect to %s' % (
      internal_gclient_path, external_gclient_path)
  f = open(internal_gclient_path, 'w')
  f.write('"%s" %%*' % external_gclient_path)
  f.close()
