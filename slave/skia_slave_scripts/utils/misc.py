#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains tools used by the buildbot scripts. """

import os
import re
import shlex
import subprocess
import sys
import threading
import time

if 'nt' in os.name:
  import ctypes

SUBPROCESS_TIMEOUT = 30.0
PATH_TO_ADB = os.path.join('..', 'android', 'bin', 'linux', 'adb')
PROCESS_MONITOR_INTERVAL = 5.0 # Seconds
SKIA_RUNNING = 'running'
SKIA_RETURN_CODE_REPEATS = 10
DEFAULT_SECS_BETWEEN_ATTEMPTS = 10
DEVICE_LOOKUP = {'nexus_s': 'crespo',
                 'xoom': 'stingray',
                 'galaxy_nexus': 'toro',
                 'nexus_7': 'grouper'}
CPU_SCALING_MODES = ['performance', 'interactive']

def ArgsToDict(argv):
  """ Collect command-line arguments of the form '--key value' into a
  dictionary.  Fail if the arguments do not fit this format. """
  dict = {}
  PREFIX = '--'
  # Expect the first arg to be the path to the script, which we don't want.
  argv = argv[1:]
  while argv:
    if argv[0].startswith(PREFIX):
      dict[argv[0][len(PREFIX):]] = argv[1]
      argv = argv[2:]
    else:
      raise Exception('Malformed input: %s' % argv)
  return dict

def ConfirmOptionsSet(name_value_dict):
  """Raise an exception if any of the given command-line options were not set.

  name_value_dict: dictionary mapping option names to option values
  """
  for (name, value) in name_value_dict.iteritems():
    if value is None:
      raise Exception('missing command-line option %s; rerun with --help' %
                      name)

def BashAsync(cmd, echo=True, shell=False):
  """ Run 'cmd' in a subprocess, returning a Popen class instance referring to
  that process.  (Non-blocking) """
  if echo:
    print cmd
  if 'nt' in os.name:
    # Windows has a bad habit of opening a dialog when a console program
    # crashes, rather than just letting it crash.  Therefore, when a program
    # crashes on Windows, we don't find out until the build step times out.
    # This code prevents the dialog from appearing, so that we find out
    # immediately and don't waste time waiting around.
    SEM_NOGPFAULTERRORBOX = 0x0002
    ctypes.windll.kernel32.SetErrorMode(SEM_NOGPFAULTERRORBOX)
    flags = 0x8000000 # CREATE_NO_WINDOW
  else:
    flags = 0
  return subprocess.Popen(cmd, shell=shell, stderr=subprocess.STDOUT,
                          stdout=subprocess.PIPE, creationflags=flags)

def LogProcessToCompletion(proc, echo=True, timeout=None):
  """ Log the output of proc until it completes. Return a tuple containing the
  exit code of proc and the contents of stdout.

  proc: an instance of Popen referring to a running subprocess.
  echo: boolean indicating whether to print the output received from proc.stdout
  timeout: number of seconds allotted for the process to run
  """
  all_output = []
  t_0 = time.time()
  while True:
    code = proc.poll()
    output = proc.stdout.read(1)
    # stdout.read(1) returns an empty string if there is nothing to read. This
    # may occur because the program has not written to stdout since the last
    # read, or because the program has finished running. The loop ends when the
    # subprocess has completed *and* we have read all of its output.
    if output == '' and code != None:
      break
    else:
      if echo:
        sys.stdout.write(output)
        sys.stdout.flush()
      all_output.append(output)
    if timeout and time.time() - t_0 > timeout:
      break
  return (code, ''.join(all_output))

def Bash(cmd, echo=True, shell=False, timeout=None):
  """ Run 'cmd' in a shell and return the combined contents of stdout and
  stderr (Blocking).  Throws an exception if the command exits non-zero.
  
  cmd: list of strings (or single string, iff shell==True) indicating the
      command to run
  echo: boolean indicating whether we should print the command and log output
  shell: boolean indicating whether we are using advanced shell features. Use
      only when absolutely necessary, since this allows a lot more freedom which
      could be exploited by malicious code. See the warning here:
      http://docs.python.org/library/subprocess.html#popen-constructor
  timeout: optional, integer indicating the maximum elapsed time in seconds
  """
  proc = BashAsync(cmd, echo=echo, shell=shell)
  (returncode, output) = LogProcessToCompletion(proc, echo=echo,
                                                timeout=timeout)
  if returncode != 0:
    raise Exception('Command failed with code %d' % returncode)
  return output

def BashRetry(cmd, echo=True, shell=False, attempts=1,
              secs_between_attempts=DEFAULT_SECS_BETWEEN_ATTEMPTS):
  """ Wrapper for Bash() which makes multiple attempts until either the command
  succeeds or the maximum number of attempts is reached. """
  attempt = 1
  while True:
    try:
      return Bash(cmd, echo=echo, shell=shell)
    except:
      if attempt >= attempts:
        raise
    print 'Command failed. Retrying in %d seconds...' % secs_between_attempts
    time.sleep(secs_between_attempts)
    attempt += 1

def RunADB(serial, cmd, echo=True, attempts=5, secs_between_attempts=10):
  """ Run 'cmd' on an Android device, using ADB.  No return value; throws an
  exception if the command fails more than the allotted number of attempts.
  
  serial: string indicating the serial number of the target device
  cmd: string; the command to issue on the device
  attempts: number of times to attempt the command
  """
  adb_cmd = [PATH_TO_ADB, '-s', serial]
  adb_cmd += cmd
  BashRetry(adb_cmd, echo=echo, attempts=attempts,
            secs_between_attempts=secs_between_attempts)

def ADBKill(serial, process):
  """ Kill a process running on an Android device.
  
  serial: string indicating the serial number of the target device
  process: string indicating the name of the process to kill
  """ 
  cmd = '%s -s %s shell ps | grep %s' % (PATH_TO_ADB, serial, process)
  try:
    stdout = Bash(cmd)
  except:
    return
  if stdout != '':
    pid = shlex.split(stdout)[1]
    kill_cmd = ['shell', 'kill', pid]
    RunADB(serial, kill_cmd)

def GetAbsPath(relative_path):
    """My own implementation of os.path.abspath() that better handles paths
    which approach Window's 260-character limit.
    See https://code.google.com/p/skia/issues/detail?id=674

    This implementation adds path components one at a time, resolving the
    absolute path each time, to take advantage of any chdirs into outer
    directories that will shorten the total path length.

    TODO: share a single implementation with bench_graph_svg.py, instead
    of pasting this same code into both files."""
    if os.path.isabs(relative_path):
        return relative_path
    path_parts = relative_path.split(os.sep)
    abs_path = os.path.abspath('.')
    for path_part in path_parts:
        abs_path = os.path.abspath(os.path.join(abs_path, path_part))
    return abs_path

def GetSerial(device_type):
  """ Determine the serial number of the *first* connected device with the
  specified type.  The ordering of 'adb devices' is not documented, and the
  connected devices do not appear to be ordered by serial number.  Therefore,
  we have to assume that, in the case of multiple devices of the same type being
  connected to one host, we cannot predict which device will be chosen.
  
  device_type: string indicating the 'common name' for the target device
  """
  if not device_type in DEVICE_LOOKUP:
    raise ValueError('Unknown device: %s!' % device_type)
  device_name = DEVICE_LOOKUP[device_type]
  output = BashRetry('%s devices' % PATH_TO_ADB, attempts=5)
  print output
  lines = output.split('\n')
  device_ids = []
  for line in lines:
    # Filter garbage lines
    if line != '' and not ('List of devices attached' in line) and \
        line[0] != '*':
      device_ids.append(line.split('\t')[0])
  for id in device_ids:
    print 'Finding type for id %s' % id
    # Get device name
    name_line = BashRetry(
        '%s -s %s shell cat /system/build.prop | grep "ro.product.device="' % (
            PATH_TO_ADB, id), attempts=5)
    print name_line
    name = name_line.split('=')[-1].rstrip()
    # Just return the first attached device of the requested model.
    if device_name in name:
      return id
  raise Exception('No %s device attached!' % device_name)

def SetCPUScalingMode(serial, mode):
  """ Set the CPU scaling governor for the device with the given serial number
  to the given mode.

  serial: string indicating the serial number of the device whose scaling mode
          is to be modified
  mode:   string indicating the desired CPU scaling mode.  Acceptable values
          are listed in CPU_SCALING_MODES.
  """
  if mode not in CPU_SCALING_MODES:
    raise ValueError('mode must be one of: %s' % CPU_SCALING_MODES)
  cpu_dirs = Bash('%s -s %s shell ls /sys/devices/system/cpu' % (
      PATH_TO_ADB, serial), echo=False, shell=True)
  cpu_dirs_list = cpu_dirs.split('\n')
  regex = re.compile('cpu\d')
  for dir in cpu_dirs_list:
    cpu_dir = dir.rstrip()
    if regex.match(cpu_dir):
      path = '/sys/devices/system/cpu/%s/cpufreq/scaling_governor' % cpu_dir
      path_found = Bash('%s -s %s shell ls %s' % (PATH_TO_ADB, serial, path),
                        echo=False, shell=True).rstrip()
      if path_found == path:
        # Unfortunately, we can't directly change the scaling_governor file over
        # ADB. Instead, we write a script to do so, push it to the device, and
        # run it.
        old_mode = Bash('%s -s %s shell cat %s' % (PATH_TO_ADB, serial, path),
                        echo=False, shell=True).rstrip()
        print 'Current scaling mode for %s is: %s' % (cpu_dir, old_mode)
        filename = 'skia_cpuscale.sh'
        with open(filename, 'w') as script_file:
          script_file.write('echo %s > %s\n' % (mode, path))
        os.chmod(filename, 0777)
        RunADB(serial, ['push', filename, '/system/bin'], echo=False)
        RunADB(serial, ['shell', filename], echo=True)
        RunADB(serial, ['shell', 'rm', '/system/bin/%s' % filename], echo=False)
        os.remove(filename)
        new_mode = Bash('%s -s %s shell cat %s' % (PATH_TO_ADB, serial, path),
                        echo=False, shell=True).rstrip()
        print 'New scaling mode for %s is: %s' % (cpu_dir, new_mode)

class _WatchLog(threading.Thread):
  """ Run WatchLog in a new thread to record the logcat output from SkiaAndroid.
  Returns iff a 'SKIA_RETURN_CODE' appears in the log, setting the return code
  appropriately.  Note that this will not terminate if the SkiaAndroid process
  does not finish normally, so we need to periodically check that the process is
  still running and terminate this thread if the process has died without
  printing a 'SKIA_RETURN_CODE'. """
  def __init__(self, serial, log_file=None):
    threading.Thread.__init__(self)
    self.retcode = SKIA_RUNNING
    self.serial = serial
    self._stopped = False
    self._log_file = log_file
    self._log_process = None
    self._mutex = threading.Lock()

  def _restart(self):
    self._mutex.acquire()
    try:
      if self._log_process:
        self._log_process.terminate()
        self._log_process = None
      self._stopped = False
      # Clear the log so we don't see a bunch of old data
      Bash('%s -s %s logcat -c' % (PATH_TO_ADB, self.serial), echo=False)
      self._log_process = BashAsync('%s -s %s logcat' % (
          PATH_TO_ADB, self.serial), echo=False, shell=True)
    finally:
      self._mutex.release()

  def stop(self):
    self._mutex.acquire()
    try:
      self._stopped = True
      self._log_process.terminate()
      self._log_process = None
    finally:
      self._mutex.release()

  def run(self):
    self._restart()
    while not self._stopped:
      line = self._log_process.stdout.readline()
      if line != '':
        if self._log_file:
          self._log_file.write(line)
        print line.rstrip('\r\n')
        if 'SKIA_RETURN_CODE' in line:
          self.retcode = shlex.split(line)[-1]
          self.stop()
          return
      elif not self._stopped:
        """ We only get an empty string from readline() when the logcat process
        has stopped running.  Otherwise, readline() blocks while waiting for
        data.  If the logcat process has died but we didn't kill it, we need to
        restart it. """
        print '**** Logcat process has died; restarting. ***'
        self._restart()

def Install(serial, path_to_apk):
  try:
    RunADB(serial, ['uninstall', 'com.skia'])
  except:
    pass
  RunADB(serial, ['install', path_to_apk])

def Run(serial, binary_name, arguments=[], logfile=None):
  """ Run 'binary_name', on the device with id 'serial', with 'arguments'.  This
  function sets and runs the Skia APK on a connected device.  We launch WatchLog
  in a new thread and then keep polling the device to make sure that the process
  is still running.  We are done when either:
  
  1. WatchLog sets a value in 'retcode'
  2. WatchLog has not set a value in 'retcode' and the Skia process has died.
  
  We then return success or failure.
  
  serial: string indicating the serial number of the target device
  binary_name: string indicating name of the program to run on the device
  arguments: string containing the arguments to pass to the program
  """
  # First, kill any running instances of the app.
  ADBKill(serial, 'skia_native')
  ADBKill(serial, 'skia')

  logger = _WatchLog(serial, log_file=logfile)
  logger.start()
  cmd_line = binary_name
  for arg in arguments:
    cmd_line = '%s %s' % (cmd_line, arg)
  cmd_line = '"%s"' % cmd_line
  RunADB(serial, ['shell', 'am', 'broadcast',
                  '-a', 'com.skia.intent.action.LAUNCH_SKIA',
                  '-n', 'com.skia/.SkiaReceiver',
                  '-e', 'args'] + shlex.split(cmd_line) +
                 ['--ei', 'returnRepeats', '%d' % SKIA_RETURN_CODE_REPEATS])
  while logger.isAlive() and logger.retcode == SKIA_RUNNING:
    time.sleep(PROCESS_MONITOR_INTERVAL)
    # adb does not always return in a timely fashion.  Don't wait for it.
    monitor = Bash(
        '%s -s %s shell ps | grep skia_native' % (PATH_TO_ADB, serial),
        echo=False, timeout=SUBPROCESS_TIMEOUT)
    if not monitor[0]: # adb timed out
      continue
    # No SKIA_RETURN_CODE printed, but the process isn't running
    if monitor[1] == '' and logger.retcode == SKIA_RUNNING:
      logger.stop()
      raise Exception('Skia process died while executing %s' % binary_name)
  if not logger.retcode == '0':
    raise Exception('Failure in %s' % binary_name)
