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


CHROME_BUILD_URL = 'https://chromium.googlesource.com/chromium/tools/build.git'
CHROME_BUILD_INTERNAL_URL = (
    'https://chrome-internal.googlesource.com/chrome/tools/build.git')
SKIA_URL = 'https://skia.googlesource.com/buildbot.git'
GCLIENT = 'gclient.bat' if os.name == 'nt' else 'gclient'
GIT = 'git.bat' if os.name == 'nt' else 'git'
NEW_MASTER_NAME = {
  'Skia': 'Skia',
  'AndroidSkia': 'SkiaAndroid',
  'CompileSkia': 'SkiaCompile',
  'FYISKIA': 'SkiaFYI',
  'PrivateSkia': 'SkiaInternal',
}
UPSTREAM_MASTER_PREFIX = 'client.skia'


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
    output = subprocess.check_output(cmd, stdout=subprocess.PIPE,
                                     stderr=subprocess.STDOUT)
    is_running = pid in output
  else:
    cmd = ['cat', '/proc/%s/stat' % pid]
    is_running = subprocess.call(cmd, stdout=subprocess.PIPE,
                                 stderr=subprocess.STDOUT) == 0
  return is_running


class BuildSlaveManager(multiprocessing.Process):
  """ Manager process for BuildSlaves. Periodically checks that any
  keepalive_conditions are met and kills or starts the slave accordingly. """

  def __init__(self, slavename, checkout_path, copies, copy_src_dir,
               master_name, keepalive_conditions, poll_interval):
    """ Construct the BuildSlaveManager.

    slavename: string; the name of the slave to start.
    checkout_path: string; the directory in which to launch the slave.
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
    self._checkout_path = checkout_path
    self._copies = copies
    self._copy_src_dir = os.path.abspath(copy_src_dir)
    self._keepalive_conditions = keepalive_conditions
    self._poll_interval = poll_interval
    self._master_name = master_name
    multiprocessing.Process.__init__(self)

  def _GClientConfig(self):
    """Run 'gclient config'."""
    subprocess.check_call([GCLIENT, 'config', SKIA_URL])

  def _SyncSources(self):
    """ Run 'gclient sync' on the buildbot sources. """
    # Check out or update the buildbot scripts
    self._GClientConfig()
    subprocess.check_call([GCLIENT, 'sync', '-j1', '--force'])

    if os.name == 'nt':
      os.environ['WIN_TOOLS_FORCE'] = '1'
      subprocess.check_call([os.path.join(os.getcwd(), 'buildbot',
                                          'third_party', 'depot_tools',
                                          GCLIENT)])
      del os.environ['WIN_TOOLS_FORCE']

    # Perform Copies
    if self._copies:
      for copy in self._copies:
        src = os.path.join(self._copy_src_dir, os.path.normpath(copy['source']))
        dest = os.path.normpath(copy['destination'])
        print 'Copying %s to %s' % (src, dest)
        shutil.copy(src, dest)

  @property
  def master_host(self):
    """Return the hostname of the master for this buildslave."""
    return config.Master.set_active_master(self._master_name).master_host

  @property
  def slave_dir(self):
    """Directory in which to launch the slave."""
    return os.path.join('buildbot', 'slave')

  def _LaunchSlave(self):
    """ Launch the BuildSlave. """
    self._KillSlave()

    self._SyncSources()

    if os.name == 'nt':
      # We run different commands for the Windows shell
      cmd = 'setlocal&&'
      cmd += 'set TESTING_SLAVENAME=%s&&' % self._slavename
      cmd += 'set TESTING_MASTER=%s&&' % self._master_name
      if self.master_host:
        cmd += 'set TESTING_MASTER_HOST=%s&&' % self.master_host
      cmd += 'run_slave.bat'
      cmd += '&& endlocal'
    else:
      cmd = 'TESTING_SLAVENAME=%s ' % self._slavename
      cmd += 'TESTING_MASTER=%s ' % self._master_name
      if self.master_host:
        cmd += 'TESTING_MASTER_HOST=%s ' % self.master_host
      cmd += 'make start'
    print 'Running cmd: %s' % cmd
    subprocess.check_call(cmd, shell=True, cwd=self.slave_dir)

    start_time = time.time()
    while not self._IsRunning():
      if time.time() - start_time > PID_TIMEOUT:
        raise Exception('Failed to launch %s' % self._slavename)
      time.sleep(1)

  def _IsRunning(self):
    """ Determine if this BuildSlave is running. If so, return its PID,
    otherwise, return None. """
    if os.path.isfile(PID_FILE):
      with open(PID_FILE) as f:
        pid = str(f.read()).rstrip()
      if IsRunning(pid):
        return pid
    return None

  def _KillSlave(self):
    """ Kill the BuildSlave. """
    pid = self._IsRunning()
    if not pid:
      print 'BuildSlaveManager._KillSlave: Slave not running.'
      return
    if os.name == 'nt':
      cmd = ['taskkill', '/F', '/T', '/PID', str(pid)]
    else:
      cmd = ['make', 'stop']
    subprocess.check_call(cmd, cwd=os.path.join('buildbot', 'slave'))

  def run(self):
    """ Run the BuildSlaveManager. This overrides multiprocessing.Process's
    run() method. """
    os.chdir(self._checkout_path)
    self._SyncSources()
    self._checkout_path = os.path.abspath(os.curdir)
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


class ChromeBuildSlaveManager(BuildSlaveManager):
  """BuildSlaveManager for slaves using Chromium build code."""

  def _GClientConfig(self):
    """Run 'gclient config'."""
    solutions = [
      { 'name': 'build',
        'url': CHROME_BUILD_URL,
        'deps_file': '.DEPS.git',
        'managed': True,
        'custom_deps': {},
        'safesync_url': '',
      },
      { 'name': 'build_internal',
        'url': CHROME_BUILD_INTERNAL_URL,
        'deps_file': '.DEPS.git',
        'managed': True,
        'custom_deps': {},
        'safesync_url': '',
      },
    ]
    cmd = [GCLIENT, 'config', '--spec=solutions=%s' % repr(solutions)]
    print 'Running command: %s' % ' '.join(cmd)
    subprocess.check_call(cmd)

  @property
  def master_host(self):
    """Return the hostname of the master for this buildslave."""
    return None  # Just use the default.

  @property
  def slave_dir(self):
    """Directory from which to launch the buildslave."""
    return os.path.join('build', 'slave')


def ReadSlavesCfg(slaves_cfg_path):
  """Read the given slaves.cfg path and return the slaves dict."""
  cfg = {}
  execfile(slaves_cfg_path, cfg)
  return cfg['slaves']


def RunSlave(slavename, connects_to_new_master=False):
  """ Launch a single slave, checking out the buildbot tree if necessary.

  slavename: string indicating the hostname of the build slave to launch.
  copies: dictionary with 'source' and 'destination' keys whose values are the
      current location and destination location within the buildbot checkout of
      files to be copied.
  """
  print 'Starting slave: %s%s' % (
      slavename, ' (new)' if connects_to_new_master else '')
  start_dir = os.path.realpath(os.curdir)
  slave_dir = os.path.join(start_dir, slavename)
  copies = (slave_hosts_cfg.CHROMEBUILD_COPIES if connects_to_new_master
            else slave_hosts_cfg.DEFAULT_COPIES)

  # Create the slave directory if needed
  if not os.path.isdir(slave_dir):
    print 'Creating directory: %s' % slave_dir
    os.makedirs(slave_dir)

  # Find the slave config dict and BuildSlaveManager type for this slave.
  slave_cfg = {}
  manager = (ChromeBuildSlaveManager if connects_to_new_master
             else BuildSlaveManager)
  for cfg in slaves_cfg.SLAVES:
    if cfg['hostname'] == slavename:
      slave_cfg = cfg
      break
  if not slave_cfg:
    # Try looking at upstream masters.
    upstream_masters_dir = os.path.join('third_party',
                                        'chromium_buildbot_tot',
                                        'masters')
    for master_dir in os.listdir(upstream_masters_dir):
      if master_dir.startswith('master.%s' % UPSTREAM_MASTER_PREFIX):
        slaves_cfg_path = os.path.join(
            upstream_masters_dir, master_dir, 'slaves.cfg')
        for cfg in ReadSlavesCfg(slaves_cfg_path):
          if cfg['hostname'] == slavename:
            slave_cfg = cfg
            break
  if not slave_cfg:
    raise Exception('No buildslave config found for %s!' % slavename)

  # Launch the buildslave.
  master_name = slave_cfg['master']
  if connects_to_new_master:
    master_name = NEW_MASTER_NAME[master_name]
  manager(slavename, slave_dir, copies, os.pardir, master_name,
          slave_cfg.get('keepalive_conditions', []), DEFAULT_POLL_INTERVAL
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


def main():
  """ Launch local build slave instances """
  # Gather command-line arguments.
  ParseArgs(sys.argv[1:])

  # Sync the buildbot code.
  subprocess.check_call([GCLIENT, 'sync', '--force', '-j1'])

  # Obtain configuration information about this build slave host machine.
  slave_host = slave_hosts_cfg.get_slave_host_config(socket.gethostname())
  slaves = slave_host.slaves
  print 'Attempting to launch build slaves:'
  for slavename, _, connects_to_new_master in slaves:
    print '  %s%s' % (slavename,
                      (' (new master)' if connects_to_new_master else ''))

  # Launch the build slaves
  for slavename, _, connects_to_new_master in slaves:
    RunSlave(slavename, connects_to_new_master)


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
