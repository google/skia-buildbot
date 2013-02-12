#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains tools used by Android-specific buildbot scripts. """

import os
import re
import shell_utils
import shlex
import sys
import threading
import time


CPU_SCALING_MODES = ['performance', 'interactive']
DEVICE_LOOKUP = {'nexus_s': 'crespo',
                 'xoom': 'stingray',
                 'galaxy_nexus': 'toro',
                 'nexus_4': 'mako',
                 'nexus_7': 'grouper',
                 'nexus_10': 'manta'}
PROCESS_MONITOR_INTERVAL = 5.0 # Seconds
SKIA_RUNNING = 'running'
SKIA_RETURN_CODE_REPEATS = 10
SUBPROCESS_TIMEOUT = 30.0


def GotADB(adb):
  """ Returns True iff ADB exists at the given location.

  adb: string; possible path to the ADB executable.
  """
  try:
    shell_utils.Bash([adb, 'version'], echo=False)
    return True
  except Exception:
    return False


def FindADB(hint=None):
  """ Attempt to find the ADB program using the following sequence of steps.
  Returns the path to ADB if it can be found, or None otherwise.
  1. If a hint was provided, is it a valid path to ADB?
  2. Is ADB in PATH?
  3. Is there an environment variable for ADB?
  4. If the ANDROID_SDK_ROOT variable is set, try to find ADB in the SDK
     directory.
  5. Try to find ADB in a list of common locations.

  hint: string indicating a possible path to ADB.
  """
  # 1. If a hint was provided, does it point to ADB?
  if hint:
    if os.path.basename(hint) == 'adb':
      adb = hint
    else:
      adb = os.path.join(hint, 'adb')
    if GotADB(adb):
      return adb

  # 2. Is 'adb' in our PATH?
  adb = 'adb'
  if GotADB(adb):
    return adb

  # 3. Is there an environment variable for ADB?
  adb = os.environ.get('ADB')
  if GotADB(adb):
    return adb

  # 4. If ANDROID_SDK_ROOT is set, try to find ADB in the SDK directory.
  sdk_dir = os.environ.get('ANDROID_SDK_ROOT', '')
  adb = os.path.join(sdk_dir, 'platform-tools', 'adb')
  if GotADB(adb):
    return adb

  # 4. Try to find ADB relative to this file.
  common_locations = []
  os_dir = None
  if sys.platform.startswith('linux'):
    os_dir = 'linux'
  elif sys.platform.startswith('darwin'):
    os_dir = 'mac'
  else:
    os_dir = 'win'
  common_locations.append(os.path.join(os.pardir, os_dir, 'adb'))
  common_locations.append(os.path.join(os.environ.get('HOME', ''),
                          'android-sdk-%s' % os_dir))
  for location in common_locations:
    if GotADB(location):
      return location

  raise Exception('android_utils: Unable to find ADB!')


PATH_TO_ADB = FindADB(hint=os.path.join('..', 'android', 'bin', 'linux', 'adb'))


def RunADB(serial, cmd, echo=True, attempts=5, secs_between_attempts=10,
           timeout=None):
  """ Run 'cmd' on an Android device, using ADB.  No return value; throws an
  exception if the command fails more than the allotted number of attempts.
  
  serial: string indicating the serial number of the target device
  cmd: string; the command to issue on the device
  attempts: number of times to attempt the command
  secs_between_attempts: number of seconds to wait between attempts
  timeout: optional, integer indicating the maximum elapsed time in seconds
  """
  adb_cmd = [PATH_TO_ADB, '-s', serial]
  adb_cmd += cmd
  shell_utils.BashRetry(adb_cmd, echo=echo, attempts=attempts,
                        secs_between_attempts=secs_between_attempts)


def ADBShell(serial, cmd, echo=True):
  """ Runs 'cmd' in the ADB shell on an Android device and returns the exit
  code.

  serial: string indicating the serial number of the target device
  cmd: string; the command to issue on the device
  """
  # ADB doesn't exit with the exit code of the command we ran. It only exits
  # non-zero when ADB itself encountered a problem. Therefore, we have to use
  # the shell to print the exit code for the command and parse that from stdout.
  adb_cmd = '%s -s %s shell "%s; echo \$?"' % (PATH_TO_ADB, serial,
                                               ' '.join(cmd))
  output = shell_utils.Bash(adb_cmd, shell=True, echo=echo)
  output_lines = output.splitlines()
  try:
    real_exitcode = int(output_lines[-1].rstrip())
  except ValueError:
    real_exitcode = -1
  if real_exitcode != 0:
    raise Exception('Command failed with code %s' % real_exitcode)
  return '\n'.join(output_lines[:-1])


def ADBKill(serial, process, kill_app=False):
  """ Kill a process running on an Android device.
  
  serial: string indicating the serial number of the target device
  process: string indicating the name of the process to kill
  kill_app: bool indicating whether the process is an Android app, as opposed
      to a normal executable process.
  """ 
  if kill_app:
    ADBShell(serial, ['am', 'kill', process])
  else:
    try:
      stdout = shell_utils.Bash('%s -s %s shell ps | grep %s' % (
                                    PATH_TO_ADB, serial, process), shell=True)
    except Exception:
      return
    for line in stdout.split('\n'):
      if line != '':
        split = shlex.split(line)
        if len(split) < 2:
          continue
        pid = split[1]
        ADBShell(serial, ['kill', pid])


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
  output = shell_utils.BashRetry('%s devices' % PATH_TO_ADB, shell=True,
                                 attempts=5)
  print output
  lines = output.split('\n')
  device_ids = []
  for line in lines:
    # Filter garbage lines
    if line != '' and not ('List of devices attached' in line) and \
        line[0] != '*':
      device_ids.append(line.split('\t')[0])
  for device_id in device_ids:
    print 'Finding type for id %s' % device_id
    # Get device name
    name_line = shell_utils.BashRetry(
        '%s -s %s shell cat /system/build.prop | grep "ro.product.device="' % (
            PATH_TO_ADB, device_id), shell=True, attempts=5)
    print name_line
    name = name_line.split('=')[-1].rstrip()
    # Just return the first attached device of the requested model.
    if device_name in name:
      return device_id
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
  cpu_dirs = shell_utils.Bash('%s -s %s shell ls /sys/devices/system/cpu' % (
      PATH_TO_ADB, serial), echo=False, shell=True)
  cpu_dirs_list = cpu_dirs.split('\n')
  regex = re.compile('cpu\d')
  for cpu_dir_from_list in cpu_dirs_list:
    cpu_dir = cpu_dir_from_list.rstrip()
    if regex.match(cpu_dir):
      path = '/sys/devices/system/cpu/%s/cpufreq/scaling_governor' % cpu_dir
      path_found = shell_utils.Bash('%s -s %s shell ls %s' % (
                                        PATH_TO_ADB, serial, path),
                                    echo=False, shell=True).rstrip()
      if path_found == path:
        # Unfortunately, we can't directly change the scaling_governor file over
        # ADB. Instead, we write a script to do so, push it to the device, and
        # run it.
        old_mode = shell_utils.Bash('%s -s %s shell cat %s' % (
                                        PATH_TO_ADB, serial, path),
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
        new_mode = shell_utils.Bash('%s -s %s shell cat %s' % (
                                        PATH_TO_ADB, serial, path),
                                    echo=False, shell=True).rstrip()
        print 'New scaling mode for %s is: %s' % (cpu_dir, new_mode)


def IsAndroidShellRunning(serial):
  """ Find the status of the Android shell for the device with the given serial
  number. Returns True if the shell is running and False otherwise.

  serial: string indicating the serial number of the target device.
  """
  if 'Error:' in ADBShell(serial, ['pm', 'path', 'android'], echo=False):
    return False
  return True


def StopShell(serial, timeout=60):
  """ Halt the Android runtime on the device with the given serial number.
  Blocks until the shell reports that it has stopped.

  serial: string indicating the serial number of the target device.
  timeout: maximum allotted time, in seconds.
  """
  ADBShell(serial, ['stop'])
  start_time = time.time()
  while IsAndroidShellRunning(serial):
    time.sleep(1)
    if time.time() - start_time > timeout:
      raise Exception('Timeout while attempting to stop the Android runtime.')


def StartShell(serial, timeout=60):
  """ Start the Android runtime on the device with the given serial number.
  Blocks until the shell reports that it has started.

  serial: string indicating the serial number of the target device.
  timeout: maximum allotted time, in seconds.
  """
  ADBShell(serial, ['start'])
  start_time = time.time()
  while not IsAndroidShellRunning(serial):
    time.sleep(1)
    if time.time() - start_time > timeout:
      raise Exception('Timeout while attempting to start the Android runtime.')


def IsSkiaAndroidAppInstalled(serial):
  """ Determine whether the Skia Android app is installed. """
  return bool('com.skia' in ADBShell(serial, ['pm', 'list', 'packages'],
                                     echo=False))


def Install(serial, release_mode=False, install_launcher=True):
  """ Install an Android app to the device with the given serial number.

  serial: string indicating the serial number of the target device.
  release_mode: bool; whether the app was build in Release mode.
  install_launcher: bool; whether or not the skia_launcher should be installed
    into /system/bin on the device.  Requires root access.
  """
  # The shell must be running to install/uninstall apps
  StartShell(serial)
  # Assuming we're in the 'trunk' directory.
  cmd = [os.path.join(os.pardir, 'android', 'bin', 'android_install_skia'),
         '-f',
         '-s', serial]
  if install_launcher:
    cmd.append('--install-launcher')
  if release_mode:
    cmd.append('--release')
  shell_utils.Bash(' '.join(cmd), shell=True)
  if not IsSkiaAndroidAppInstalled(serial):
    raise Exception('Failed to install Skia Android app.')
  if install_launcher:
    try:
      ADBShell(serial, ['ls', '/system/bin/skia_launcher'])
    except Exception:
      raise Exception('Failed to push skia_launcher.')


def RunSkia(serial, cmd, use_intent=False, stop_shell=True):
  """ Run the given Skia executable command on an Android device.

  serial: string indicating the serial number of the target device.
  cmd: list of strings; the command to run
  use_intent: bool; whether or not to use a broadcast intent to launch the
      command in the Skia Android app.
  stop_shell: bool; whether or not to stop the Android framework.
  """
  if use_intent:
    return RunSkiaIntent(serial, cmd)
  else:
    return RunSkiaShell(serial, cmd, stop_shell)


def RunSkiaShell(serial, cmd, stop_shell=True):
  """ Run the given command through skia_launcher on a given device.

  serial: string indicating the serial number of the target device.
  cmd: list of strings; the command line to run.
  stop_shell: bool; whether or not to stop the Android framework.
  """
  if stop_shell:
    StopShell(serial)
  RunADB(serial, ['logcat', '-c'])
  try:
    ADBShell(serial, ['skia_launcher'] + cmd)
  finally:
    RunADB(serial, ['logcat', '-d', '-v', 'time'])


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
    print '_WatchLog.restart()'
    self._mutex.acquire()
    try:
      if self._log_process:
        self._log_process.terminate()
        self._log_process = None
      self._stopped = False
      # Clear the log so we don't see a bunch of old data
      RunADB(self.serial, ['logcat', '-c'], echo=False)
      self._log_process = shell_utils.BashAsync('%s -s %s logcat' % (
          PATH_TO_ADB, self.serial), echo=False, shell=True)
    finally:
      self._mutex.release()

  def stop(self):
    print '_Watchlog.stop()'
    self._mutex.acquire()
    try:
      self._stopped = True
      self._log_process.terminate()
      self._log_process = None
    finally:
      self._mutex.release()

  def run(self):
    self._restart()
    output = shell_utils.LogProcessToCompletion(
        self._log_process,
        log_file=self._log_file,
        halt_on_output='SKIA_RETURN_CODE')[1]
    print 'Subprocess finished.'
    if not self._stopped:
      # If the logger was stopped, then we know the Skia process died.
      for line in output.split('\n'):
        if 'SKIA_RETURN_CODE' in line:
          self.retcode = shlex.split(line)[-1]
          return
    self.retcode = '-1'


def RunSkiaIntent(serial, cmd, logfile=None):
  """ Run 'cmd', on the device with id 'serial' using a broadcast intent.  We
  launch WatchLog in a new thread and then keep polling the device to make sure
  that the process is still running.  We are done when either:

  1. WatchLog sets a value in 'retcode'
  2. WatchLog has not set a value in 'retcode' and the Skia process has died.

  We then return success or failure.

  serial: string indicating the serial number of the target device
  cmd: list of strings; command to run on the device.
  logfile: optional, path to a log file on the device.
  """
  # First, kill any running instances of the app.
  ADBKill(serial, 'com.skia', kill_app=True)

  logger = _WatchLog(serial, log_file=logfile)
  logger.daemon = True
  logger.start()
  try:
    RunADB(serial, ['shell', 'am', 'broadcast',
                    '-a', 'com.skia.intent.action.LAUNCH_SKIA',
                    '-n', 'com.skia/.SkiaReceiver',
                    '-e', 'args', '"\"%s\""' % ' '.join(cmd),
                    '--ei', 'returnRepeats', '%d' % SKIA_RETURN_CODE_REPEATS])
  except:
    logger.stop()
    raise
  while logger.isAlive() and logger.retcode == SKIA_RUNNING:
    time.sleep(PROCESS_MONITOR_INTERVAL)
    print 'Polling Skia process...'
    # adb does not always return in a timely fashion.  Don't wait for it.
    monitor_proc = shell_utils.BashAsync(
        '%s -s %s shell ps | grep skia_native' % (PATH_TO_ADB, serial),
        echo=False, shell=True)
    retcode, output = shell_utils.LogProcessToCompletion(monitor_proc,
        echo=False, timeout=SUBPROCESS_TIMEOUT)
    if retcode is None: # adb timed out
      print 'Poller timed out.'
      continue
    # No SKIA_RETURN_CODE printed, but the process isn't running
    if (retcode != 0 or output == '') and logger.retcode == SKIA_RUNNING:
      logger.stop()
      raise Exception('Skia process died while executing: %s' % ' '.join(cmd))
  if not logger.retcode == '0':
    raise Exception('Command failed: %s' % ' '.join(cmd))
  print 'RunSkiaIntent: Done.'
