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
import tempfile
import time


buildbot_path = os.path.abspath(os.path.join(os.path.dirname(__file__),
                                             os.pardir))
sys.path.append(os.path.join(buildbot_path, 'site_config'))

import slave_hosts_cfg


CHROME_INTERNAL = 'https://chrome-internal.googlesource.com/'
CHROME_SLAVE_DEPS_URL = CHROME_INTERNAL + 'chrome/tools/build/slave.DEPS'
CHROME_SLAVE_INTERNAL_DEPS_URL = (
    CHROME_INTERNAL + 'chrome/tools/build/internal.DEPS')
GCLIENT = 'gclient.bat' if os.name == 'nt' else 'gclient'

# How often we should check each buildslave's keepalive conditions, in seconds.
PID_FILE = os.path.join('build', 'slave', 'twistd.pid')

# Maximum time (in seconds) to wait for PID_FILE to be written after the slave
# is launched.  If PID_FILE is not written by then, we assume an error occurred.
PID_TIMEOUT = 60.0

# Cronjob entry that triggers the update of the Android toolkit. This will
# be expanded with the path of the run_daily.sh script.
CRON_ENTRY_TMPL = "00 3 * * * %s"

logger = None


def IsRunning(pid):
  """ Determine whether a process with the given PID is running.

  pid: string; the PID to test. If pid is None, return False.
  """
  if pid is None:
    return False
  if os.name == 'nt':
    cmd = ['tasklist', '/FI', '"PID eq %s"' % pid]
    output = subprocess.check_output(cmd, stdout=subprocess.PIPE,
                                     stderr=subprocess.STDOUT)
    is_running = pid in output
  else:
    cmd = ['cat', '/proc/%s/stat' % pid]
    is_running = subprocess.call(cmd, stdout=subprocess.PIPE,
                                 stderr=subprocess.STDOUT) == 0
  return is_running


def _IsRunning():
  """Determine if the BuildSlave is running in CWD

  If so, return its PID. Otherwise, return None.
  """
  if os.path.isfile(PID_FILE):
    with open(PID_FILE) as f:
      pid = str(f.read()).rstrip()
    if IsRunning(pid):
      return pid
  return None


def _KillSlave():
  """Kill the BuildSlave running in CWD."""
  pid = _IsRunning()
  if not pid:
    print '_KillSlave: Slave not running.'
    return
  if os.name == 'nt':
    cmd = ['taskkill', '/F', '/T', '/PID', str(pid)]
  else:
    cmd = ['make', 'stop']
  subprocess.check_call(cmd, cwd=os.path.join('buildbot', 'slave'))


class BuildSlaveManager(multiprocessing.Process):
  """Manager process for BuildSlaves."""

  def __init__(self, slavename, checkout_path, copies, copy_src_dir,
               is_internal):
    """ Construct the BuildSlaveManager.

    slavename: string; the name of the slave to start.
    checkout_path: string; the directory in which to launch the slave.
    copies: list of dictionaries; files to copy into the slave's source
        checkout.
    copy_src_dir: string; directory in which the files to copy reside.
    is_internal: bool; whether this buildslave uses internal code.
    """
    self._slavename = slavename
    self._checkout_path = checkout_path
    self._copies = copies
    self._copy_src_dir = os.path.abspath(copy_src_dir)
    self._is_internal = is_internal
    multiprocessing.Process.__init__(self)

  def _GClientConfig(self):
    """Run 'gclient config'."""
    config_url = (CHROME_SLAVE_INTERNAL_DEPS_URL if self._is_internal
                  else CHROME_SLAVE_DEPS_URL)
    cmd = [GCLIENT, 'config', config_url, '--deps-file', '.DEPS.git']
    print 'Running command: %s' % ' '.join(cmd)
    subprocess.check_call(cmd)

  def _SyncSources(self):
    """ Run 'gclient sync' on the buildbot sources. """
    # Check out or update the buildbot scripts
    self._GClientConfig()
    subprocess.check_call([GCLIENT, 'sync', '-j1', '--force'])

    if os.name == 'nt':
      os.environ['WIN_TOOLS_FORCE'] = '1'
      subprocess.check_call([os.path.join(os.getcwd(), 'depot_tools', GCLIENT)])
      del os.environ['WIN_TOOLS_FORCE']

    # Perform Copies
    if self._copies:
      for copy in self._copies:
        src = os.path.join(self._copy_src_dir, os.path.normpath(copy['source']))
        dest = os.path.normpath(copy['destination'])
        print 'Copying %s to %s' % (src, dest)
        shutil.copy(src, dest)

  def _LaunchSlave(self):
    """ Launch the BuildSlave. """
    _KillSlave()

    self._SyncSources()

    if os.name == 'nt':
      # We run different commands for the Windows shell
      cmd = 'setlocal&&'
      cmd += 'set TESTING_SLAVENAME=%s&&' % self._slavename
      cmd += 'run_slave.bat'
      cmd += '&& endlocal'
    else:
      cmd = 'TESTING_SLAVENAME=%s ' % self._slavename
      cmd += 'make start'
    print 'Running cmd: %s' % cmd
    subprocess.check_call(cmd, shell=True, cwd=os.path.join('build', 'slave'))

    start_time = time.time()
    while not _IsRunning():
      if time.time() - start_time > PID_TIMEOUT:
        raise Exception('Failed to launch %s' % self._slavename)
      time.sleep(1)

  def run(self):
    """ Run the BuildSlaveManager. This overrides multiprocessing.Process's
    run() method. """
    os.chdir(self._checkout_path)
    self._SyncSources()
    self._checkout_path = os.path.abspath(os.curdir)
    _KillSlave()
    self._LaunchSlave()
    print 'Successfully launched slave %s.' % self._slavename
    print 'Slave process for %s has finished.' % self._slavename


def ReadSlavesCfg(slaves_cfg_path):
  """Read the given slaves.cfg path and return the slaves dict."""
  cfg = {}
  execfile(slaves_cfg_path, cfg)
  return cfg['slaves']


def RunSlave(slavename, slave_num, is_internal):
  """ Launch a single slave, checking out the buildbot tree if necessary.

  slavename: string indicating the hostname of the build slave to launch.
  slave_num: string; the ID number of this slave on this machine. This ensures
      that particular buildslaves always run in the same place on a given
      machine.
  is_internal: bool; whether this slave uses internal code.
  """
  print 'Starting slave: %s' % slavename
  start_dir = os.path.realpath(os.curdir)
  slave_dir = os.path.join(start_dir, slavename)
  if os.name == 'nt':
    slave_dir = os.path.join('c:\\', slave_num)
  copies = slave_hosts_cfg.CHROMEBUILD_COPIES

  # Create the slave directory if needed
  if not os.path.isdir(slave_dir):
    print 'Creating directory: %s' % slave_dir
    os.makedirs(slave_dir)

  # Launch the buildslave.
  BuildSlaveManager(slavename, slave_dir, copies, os.pardir, is_internal
                    ).start()


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


def setup_cronjob():
  # We only support Mac and Linux and assume the sdk is installed in HOME.
  if sys.platform.startswith('linux'):
    sdk_dir = "android-sdk-linux"
  elif sys.platform.startswith('darwin'):
    sdk_dir = "android-sdk-macosx"
  else:
    return

  android_path = os.path.join(os.environ["HOME"], sdk_dir)
  if not os.path.exists(android_path):
    raise Exception('Android SDK not installed at %s' % android_path)

  try:
    crontab_file = ""
    c_file = tempfile.NamedTemporaryFile(delete=False)
    script_path = os.path.join(buildbot_path,"scripts", "run_daily.sh")
    c_file.write((CRON_ENTRY_TMPL % script_path) + "\n")
    c_file.close()
    crontab_file = c_file.name
    subprocess.call(["crontab", "-r"])
    subprocess.call(["crontab", crontab_file])
  finally:
    try:
      os.remove(crontab_file)
    except OSError as e:
      if e.errno != errno.EEXIST:
        raise


def main():
  """ Launch local build slave instances """
  # Gather command-line arguments.
  ParseArgs(sys.argv[1:])

  # This is needed on Windows bots syncing internal code.
  if os.name == 'nt':
    os.environ['HOME'] = os.path.join('c:\\', 'Users', 'chrome-bot')

  # Sync the buildbot code.
  subprocess.check_call([GCLIENT, 'sync', '--force', '-j1'])

  # Set up launch-on-reboot.
  launch_on_reboot = os.path.join(buildbot_path, 'scripts',
                                  'launch_on_reboot.py')
  subprocess.check_call(['python', launch_on_reboot])

  # Obtain configuration information about this build slave host machine.
  slave_host = slave_hosts_cfg.get_slave_host_config(socket.gethostname())
  slaves = slave_host.slaves
  print 'Attempting to launch build slaves:'
  for slavename, _, _ in slaves:
    print '  %s' % slavename

  # Launch the build slaves
  for slavename, slave_num, is_internal in slaves:
    RunSlave(slavename, slave_num, is_internal)

  # Set up cron jobs.
  setup_cronjob()


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
