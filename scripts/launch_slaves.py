#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Launch multiple buildbot slave instances on a single machine.  This script
is intended to be run at boot time. """


import multiprocessing
import os
import shutil
import socket
import subprocess
import sys
import time


buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir))
sys.path.append(os.path.join(buildbot_path, 'master'))
sys.path.append(os.path.join(buildbot_path, 'site_config'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'scripts'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'site_config'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'third_party', 'buildbot_8_4p1'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'third_party', 'jinja2'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'third_party', 'twisted_8_1'))


import config
import slave_hosts_cfg
import slaves_cfg


SKIA_URL = 'https://skia.googlesource.com/buildbot.git'
GCLIENT = 'gclient.bat' if os.name == 'nt' else 'gclient'
GIT = 'git.bat' if os.name == 'nt' else 'git'


# How often we should check each buildslave's keepalive conditions, in seconds.
DEFAULT_POLL_INTERVAL = 60
PID_FILE = os.path.join('buildbot', 'third_party', 'chromium_buildbot', 'slave',
                        'twistd.pid')
# Maximum time (in seconds) to wait for PID_FILE to be written after the slave
# is launched.  If PID_FILE is not written by then, we assume an error occurred.
PID_TIMEOUT = 60.0



logger = None
    

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


class BuildSlaveManager(multiprocessing.Process):
  """ Manager process for BuildSlaves. Periodically checks that any
  keepalive_conditions are met and kills or starts the slave accordingly. """

  def __init__(self, slavename, slave_dir, copies, copy_src_dir, master_name,
               keepalive_conditions, poll_interval):
    """ Construct the BuildSlaveManager.

    slavename: string; the name of the slave to start.
    slave_dir: string; the directory in which to launch the slave.
    copies: list of dictionaries; files to copy into the slave's source
        checkout.
    copy_src_dir: string; directory in which the files to copy reside.
    master_name: string; name of the master to which this build slave connects.
        This is NOT the hostname of the master, which is obtained from the
        master class in config_private.py.
    keepalive_conditions: list; commands which must succeed in order for the
        slave to stay alive.
    poll_interval: number; how often to verify the keepalive_conditions, in
        seconds.
    """
    self._slavename = slavename
    self._slave_dir = slave_dir
    self._copies = copies
    self._copy_src_dir = os.path.abspath(copy_src_dir)
    self._keepalive_conditions = keepalive_conditions
    self._poll_interval = poll_interval
    self._master_name = master_name
    multiprocessing.Process.__init__(self)

  def _SyncSources(self):
    """ Run 'gclient sync' on the buildbot sources. """
    # Check out or update the buildbot scripts
    if os.name == 'nt':
      gclient = 'gclient.bat'
    else:
      gclient = 'gclient'
    proc = subprocess.Popen([gclient, 'config', SKIA_URL])
    if proc.wait() != 0:
      raise Exception('Could not successfully configure gclient.')
    proc = subprocess.Popen([gclient, 'sync', '-j1', '--force'])
    if proc.wait() != 0:
      raise Exception('Sync failed.')

    # Perform Copies
    for copy in self._copies:
      src = os.path.join(self._copy_src_dir, os.path.normpath(copy['source']))
      dest = os.path.normpath(copy['destination'])
      print 'Copying %s to %s' % (src, dest)
      shutil.copy(src, dest)

  def _LaunchSlave(self):
    """ Launch the BuildSlave. """
    if self._IsRunning():
      self._KillSlave()

    self._SyncSources()

    # Find the hostname of the master we're connecting to.
    master_host = config.Master.get(self._master_name).master_host

    os.chdir(os.path.join('buildbot', 'slave'))
    if os.name == 'nt':
      # We run different commands for the Windows shell
      cmd = 'setlocal&&'
      cmd += 'set TESTING_SLAVENAME=%s&&' % self._slavename
      cmd += 'set TESTING_MASTER=%s&&' % self._master_name
      cmd += 'set TESTING_MASTER_HOST=%s&&' % master_host
      cmd += 'run_slave.bat'
      cmd += '&& endlocal'
    else:
      proc = subprocess.Popen(['make', 'stop'])
      proc.wait()
      cmd = 'TESTING_SLAVENAME=%s ' % self._slavename
      cmd += 'TESTING_MASTER=%s ' % self._master_name
      cmd += 'TESTING_MASTER_HOST=%s ' % master_host
      cmd += 'make start'
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
    self._SyncSources()
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

  # Create the slave directory if needed
  if not os.path.isdir(slave_dir):
    print 'Creating directory: %s' % slave_dir
    os.makedirs(slave_dir)

  # Find the slave config dict for this slave.
  slave_cfg = {}
  for cfg in slaves_cfg.SLAVES:
    if cfg['hostname'] == slavename:
      slave_cfg = cfg
      break
  if not slave_cfg:
    raise Exception('No buildslave config found for %s!' % slavename)

  manager = BuildSlaveManager(slavename, slave_dir, copies, os.pardir,
                              slave_cfg['master'],
                              slave_cfg.get('keepalive_conditions', []),
                              DEFAULT_POLL_INTERVAL)
  manager.start()


class FileLogger:
  """ Write stdout to a log file. """
  def __init__(self, log_file_name):
    # Open the log file.
    self._logfile = open(log_file_name, 'w')
    self._stdout = sys.stdout
    sys.stdout = self

  def __del__(self):
    self._logfile.close()
    sys.stdout = self._stdout

  def fileno(self):
    return self._stdout.fileno()

  def write(self, data):
    self._logfile.write(data)
    self._stdout.write(data)
    # Always flush the log file.
    self._logfile.flush()

  def flush(self):
    self._logfile.flush()
    self._stdout.flush()


def ParseArgs(argv):
  """ Parse and validate command-line arguments. """

  class CollectedArgs(object):
    pass

  usage = (
"""launch_slaves.py: Launch build slaves.
python launch_slaves.py

-h, --help          Show this message.
""")

  def Exit(error_msg=None):
    if error_msg:
      print error_msg
    print usage
    sys.exit(1)

  while argv:
    arg = argv.pop(0)
    if arg == '-h' or arg == '--help':
      Exit()
    else:
      Exit('Unknown argument: %s' % arg)
  return CollectedArgs()


def main():
  """ Launch local build slave instances """
  # Gather command-line arguments.
  ParseArgs(sys.argv[1:])

  # Sync the buildbot code.
  subprocess.check_call([GCLIENT, 'sync', '--force', '-j1'])

  # Obtain configuration information about this build slave host machine.
  slave_host = slave_hosts_cfg.GetSlaveHostConfig(socket.gethostname())
  slaves = slave_host['slaves']
  copies = slave_host['copies']
  print 'Attempting to launch build slaves:'
  for slavename, _ in slaves:
    print '  %s' % slavename

  # Launch the build slaves
  for slavename, _ in slaves:
    RunSlave(slavename, copies)


if '__main__' == __name__:
  # Pipe all output to a log file.
  logfile = 'launch_slaves.log'
  if os.path.isfile(logfile):
    num = 1
    new_filename = logfile + '.' + str(num)
    while os.path.isfile(new_filename):
      num += 1
      new_filename = logfile + '.' + str(num)
    os.rename(logfile, new_filename)
  logger = FileLogger(logfile)
  sys.exit(main())
