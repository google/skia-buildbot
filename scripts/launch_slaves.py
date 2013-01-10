#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Launch multiple buildbot slave instances on a single machine.  This script
is intended to be run at boot time. """


from contextlib import closing

import os
if os.name == 'nt':
  import win32api
  import string
import shutil
import socket
import subprocess
import sys
import urllib2


SVN_URL = 'https://skia.googlecode.com/svn/buildbot'
DRIVE_MAPPING=True


if os.name == 'nt':
  def GetFirstFreeDriveLetter():
    """ Returns the first unused Windows drive letter in [A, Z] """
    all_possible = [c for c in string.uppercase]
    in_use = win32api.GetLogicalDriveStrings()
    free = list(set(all_possible) - set(in_use))
    return free[0]


def RunSlave(slavename, copies):
  """ Launch a single slave, checking out the buildbot tree if necessary.

  slavename: string indicating the hostname of the build slave to launch.
  copies: dictionary with 'source' and 'destination' keys whose values are the
      current location and destination location within the buildbot checkout of
      files to be copied.
  """
  print 'Starting slave: %s' % slavename
  start_dir = os.path.realpath(os.curdir)
  slave_dir = os.path.join(start_dir, slavename)

  # Optionally map the slave directory to a drive letter.  This is helpful in
  # avoiding path length limits on Windows.
  if os.name == 'nt' and DRIVE_MAPPING:
    drive_letter = GetFirstFreeDriveLetter()
    print 'Mapping %c' % drive_letter
    cmd = 'net use %c: \\\\localhost\%s' % (drive_letter,
                                            slave_dir.replace(':', '$'))
    print 'Running cmd: %s' % cmd
    proc = subprocess.Popen(cmd)
    if proc.wait() != 0:
      raise Exception('Could not map %c' % drive_letter)
    # Because of weirdness in gclient, we can't run "gclient sync" in a drive
    # root.  So, we inject a minimal extra level.
    slave_dir = os.path.join('%c:' % drive_letter, 'b')

  # Create the slave directory if needed
  if not os.path.isdir(slave_dir):
    print 'Creating directory: %s' % slave_dir
    os.makedirs(slave_dir)

  # Check out or update the buildbot scripts
  os.chdir(slave_dir)
  if os.name == 'nt':
    gclient = 'gclient.bat'
  else:
    gclient = 'gclient'
  proc = subprocess.Popen([gclient, 'config', SVN_URL])
  if proc.wait() != 0:
    raise Exception('Could not successfully configure gclient.')
  proc = subprocess.Popen([gclient, 'sync', '-j1'])
  if proc.wait() != 0:
    raise Exception('Sync failed.')
  os.chdir(start_dir)

  # Perform Copies
  for copy in copies:
    src = os.path.normpath(copy['source'])
    dest = os.path.join(slave_dir, os.path.normpath(copy['destination']))
    print 'Copying %s to %s' % (src, dest)
    shutil.copy(src, dest)

  try:
    os.remove(os.path.join(slave_dir, 'buildbot', 'third_party',
                           'chromium_buildbot', 'slave', 'twistd.pid'))
  except:
    pass
  os.chdir(os.path.join(slave_dir, 'buildbot', 'slave'))
  if os.name == 'nt':
    # We run different commands for the Windows shell
    cmd = 'setlocal&&'
    cmd += 'set TESTING_SLAVENAME=%s&&' % slavename
    cmd += 'run_slave.bat'
    cmd += '&& endlocal'
  else:
    cmd = 'TESTING_SLAVENAME=%s ' % slavename
    cmd += 'make restart'
  print 'Running cmd: %s' % cmd
  subprocess.Popen(cmd, shell=True)
  os.chdir(start_dir)


def GetCfg(url):
  """ Retrieve a config file from the SVN repository and return it.
  
  url: string; the url of the file to load.
  """
  with closing(urllib2.urlopen(url)) as f:
    vars = {}
    exec(f.read(), vars)
    return vars


def GetSlaveHostCfg():
  """ Retrieve the latest slave_hosts.cfg file from the SVN repository and
  return the slave host configuration for this machine.
  """
  cfg_vars = GetCfg(SVN_URL + '/site_config/slave_hosts.cfg')
  return cfg_vars['GetSlaveHostConfig'](socket.gethostname())


def main():
  """ Launch local build slave instances """

  # Obtain configuration information about this build slave host machine.
  slave_host = GetSlaveHostCfg()
  slaves = slave_host['slaves']
  copies = slave_host['copies']

  # Launch the build slaves
  for slavename in slaves:
    RunSlave(slavename, copies)


if '__main__' == __name__:
  sys.exit(main())
