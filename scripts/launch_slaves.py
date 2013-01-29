#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Launch multiple buildbot slave instances on a single machine.  This script
is intended to be run at boot time. """


from contextlib import closing

import multiprocessing
import os
if os.name == 'nt':
  import win32api
  import string
import shutil
import socket
import subprocess
import sys
import time
import urllib2


# How often we should check each buildslave's keepalive conditions, in seconds.
DEFAULT_POLL_INTERVAL = 60
DRIVE_MAPPING = True
PID_FILE = os.path.join('buildbot', 'third_party', 'chromium_buildbot', 'slave',
                        'twistd.pid')
# Maximum time (in seconds) to wait for PID_FILE to be written after the slave
# is launched.  If PID_FILE is not written by then, we assume an error occurred.
PID_TIMEOUT = 10.0
SVN_URL = 'https://skia.googlecode.com/svn/buildbot'


if os.name == 'nt':
  def GetFirstFreeDriveLetter():
    """ Returns the first unused Windows drive letter in [A, Z] """
    all_possible = [c for c in string.uppercase]
    in_use = win32api.GetLogicalDriveStrings()
    free = list(set(all_possible) - set(in_use))
    return free[0]


# TODO(borenet): Share this code with launch_master.py.
def IsRunning(pid):
  """ Determine whether a process with the given PID is running.

  pid: string; the PID to test. If pid is None, return False.
  """
  if pid is None:
    return False
  if os.name == 'nt':
    cmd = ['tasklist', '/FI', '"PID eq %s"' % pid]
    proc = subprocess.Popen(cmd, stdout=subprocess.PIPE,
                            stderr=subprocess.STDOUT)
    if proc.wait() != 0:
      raise Exception('Unable to poll process with PID %s' % pid)
    is_running = pid in proc.communicate()[0]
  else:
    cmd = ['cat', '/proc/%s/stat' % pid]
    is_running = subprocess.Popen(cmd, stdout=subprocess.PIPE,
                                  stderr=subprocess.STDOUT).wait() == 0
  return is_running


def _SyncSources(copies):
  """ Run 'gclient sync' on the buildbot sources. """

  # Check out or update the buildbot scripts
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

  # Perform Copies
  for copy in copies:
    src = os.path.join(os.pardir, os.path.normpath(copy['source']))
    dest = os.path.normpath(copy['destination'])
    print 'Copying %s to %s' % (src, dest)
    shutil.copy(src, dest)


class BuildSlaveManager(multiprocessing.Process):
  """ Manager process for BuildSlaves. Periodically checks that any
  keepalive_conditions are met and kills or starts the slave accordingly. """

  def __init__(self, slavename, slave_dir, copies, keepalive_conditions,
               poll_interval):
    """ Construct the BuildSlaveManager.

    slavename: string; the name of the slave to start.
    slave_dir: string; the directory in which to launch the slave.
    copies: list of dictionaries; files to copy into the slave's source
        checkout.
    keepalive_conditions: list; commands which must succeed in order for the
        slave to stay alive.
    poll_interval: number; how often to verify the keepalive_conditions, in
        seconds.
    """
    self._slavename = slavename
    self._slave_dir = slave_dir
    self._copies = copies
    self._keepalive_conditions = keepalive_conditions
    self._poll_interval = poll_interval
    multiprocessing.Process.__init__(self)

  def _LaunchSlave(self):
    """ Launch the BuildSlave. """
    if self._IsRunning():
      self._KillSlave()

    _SyncSources(self._copies)

    os.chdir(os.path.join('buildbot', 'slave'))
    if os.name == 'nt':
      # We run different commands for the Windows shell
      cmd = 'setlocal&&'
      cmd += 'set TESTING_SLAVENAME=%s&&' % self._slavename
      cmd += 'run_slave.bat'
      cmd += '&& endlocal'
    else:
      cmd = 'TESTING_SLAVENAME=%s ' % self._slavename
      cmd += 'TESTING_MASTER_HOST=localhost '
      cmd += 'make restart'
    print 'Running cmd: %s' % cmd
    subprocess.Popen(cmd, shell=True)
    os.chdir(self._slave_dir)

    start_time = time.time()
    while not self._IsRunning():
      if time.time() - start_time > PID_TIMEOUT:
        raise Exception('Failed to launch %s' % self._slavename)
      time.sleep(1)

  def _IsRunning(self):
    """ Determine if this BuildSlave is running. If so, return its PID,
    otherwise, return None. """
    if os.path.isfile(PID_FILE):
      pid_file = open(PID_FILE)
      pid = str(pid_file.read()).rstrip()
      pid_file.close()
      if IsRunning(pid):
        return pid
    return None

  def _KillSlave(self):
    """ Kill the BuildSlave. """
    pid = self._IsRunning()
    if not pid:
      print 'BuildSlaveManager._KillSlave: Slave not running.'
      return
    os.chdir(os.path.join('buildbot', 'slave'))
    if os.name == 'nt':
      cmd = ['taskkill', '/F', '/T', '/PID', str(pid)]
    else:
      cmd = ['make', 'stop']
    if subprocess.Popen(cmd).wait() != 0:
      raise Exception('Failed to kill slave with pid %s' % str(pid))
    os.chdir(self._slave_dir)

  def run(self):
    """ Run the BuildSlaveManager. This overrides multiprocessing.Process's
    run() method. """
    os.chdir(self._slave_dir)
    _SyncSources(self._copies)
    self._slave_dir = os.path.abspath(os.curdir)
    if self._IsRunning():
      self._KillSlave()
    while True:
      print 'Checking keepalive conditions for %s' % self._slavename
      slave_can_run = True
      for keepalive_condition in self._keepalive_conditions:
        print 'Executing keepalive condition: %s' % keepalive_condition
        proc = subprocess.Popen(keepalive_condition, stdout=subprocess.PIPE,
                                stderr=subprocess.STDOUT)
        if proc.wait() != 0:
          print 'Keepalive condition failed for %s: %s' % (self._slavename,
                                                           keepalive_condition)
          print proc.communicate()[0]
          slave_can_run = False
          break
        print proc.communicate()[0]
      if not slave_can_run and self._IsRunning():
        self._KillSlave()
      elif slave_can_run and not self._IsRunning():
        self._LaunchSlave()
        print 'Successfully launched slave %s.' % self._slavename
      time.sleep(self._poll_interval)
    print 'Slave process for %s has finished.' % self._slavename


def RunSlave(slavename, copies, slaves_cfg):
  """ Launch a single slave, checking out the buildbot tree if necessary.

  slavename: string indicating the hostname of the build slave to launch.
  copies: dictionary with 'source' and 'destination' keys whose values are the
      current location and destination location within the buildbot checkout of
      files to be copied.
  """
  print 'Starting slave: %s' % slavename
  start_dir = os.path.realpath(os.curdir)
  slave_dir = os.path.join(start_dir, slavename)

  # Create the slave directory if needed
  if not os.path.isdir(slave_dir):
    print 'Creating directory: %s' % slave_dir
    os.makedirs(slave_dir)

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
    os.makedirs(slave_dir)

  slave_cfg = {}
  for cfg in slaves_cfg:
    if cfg['hostname'] == slavename:
      slave_cfg = cfg
      break

  manager = BuildSlaveManager(slavename, slave_dir, copies,
                              slave_cfg.get('keepalive_conditions', []), 10)
  manager.start()


def GetCfg(url):
  """ Retrieve a config file from the SVN repository and return it.
  
  url: string; the url of the file to load.
  """
  with closing(urllib2.urlopen(url)) as f:
    config_vars = {}
    exec(f.read(), config_vars)
    return config_vars


def GetSlaveHostCfg():
  """ Retrieve the latest slave_hosts.cfg file from the SVN repository and
  return the slave host configuration for this machine.
  """
  cfg_vars = GetCfg(SVN_URL + '/site_config/slave_hosts.cfg')
  return cfg_vars['GetSlaveHostConfig'](socket.gethostname())


def GetSlavesCfg():
  """ Retrieve the latest slaves.cfg file from the SVN repository. """
  cfg_vars = GetCfg(SVN_URL + '/master/slaves.cfg')
  return cfg_vars['slaves']


def main():
  """ Launch local build slave instances """

  # Obtain configuration information about this build slave host machine.
  slave_host = GetSlaveHostCfg()
  slaves = slave_host['slaves']
  copies = slave_host['copies']

  # Obtain buildslave-specific configuration information.
  slaves_cfg = GetSlavesCfg()

  # Launch the build slaves
  for slavename in slaves:
    RunSlave(slavename, copies, slaves_cfg)


if '__main__' == __name__:
  sys.exit(main())
