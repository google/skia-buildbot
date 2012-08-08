#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains tools used by the Android buildbot scripts. """

import os
import shlex
import subprocess
import threading
import time

SUBPROCESS_TIMEOUT = 30.0
PATH_TO_ADB = os.path.join('..', 'android', 'bin', 'linux', 'adb')
PATH_TO_APK = os.path.join('out', 'android', 'bin', 'SkiaAndroid.apk')
PROCESS_MONITOR_INTERVAL = 5.0 # Seconds
SKIA_RUNNING = 'running'
DEVICE_LOOKUP = {'nexus_s': 'crespo',
                 'xoom': 'stingray',
                 'galaxy_nexus': 'toro',
                 'nexus_7': 'grouper'}

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

def Bash(cmd, echo=True):
  """ Run 'cmd' in a shell and return True iff the 'cmd' succeeded.
  (Blocking) """
  if echo:
    print cmd
  return subprocess.call(cmd) == 0;

def BashGet(cmd, echo=True):
  """ Run 'cmd' in a shell and return stdout. (Blocking) """
  if echo:
    print(cmd)
  return subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True
                          ).communicate()[0]

def BashGetTimeout(cmd, echo=True, timeout=SUBPROCESS_TIMEOUT):
  """ Run 'cmd' in a shell and return the exit code.  Blocks until the command
  is finished or the timeout expires. """
  proc = BashAsync(cmd, echo=echo)
  t_0 = time.time()
  t_elapsed = 0.0
  while not proc.poll() and t_elapsed < timeout:
    time.sleep(1)
    t_elapsed = time.time() - t_0
  return proc.poll(), proc.communicate()[0]

def BashAsync(cmd, echo=True):
  """ Run 'cmd' in a subprocess, returning a handle to that process.
  (Non-blocking) """
  if echo:
    print cmd
  return subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True,
                          stderr=subprocess.STDOUT)

def RunADB(serial, cmd, attempts=1):
  """ Run 'cmd' on an Android device, using ADB
  
  serial: string indicating the serial number of the target device
  cmd: string; the command to issue on the device
  attempts: number of times to attempt the command
  """
  adb_cmd = [PATH_TO_ADB, '-s', serial]
  adb_cmd += cmd
  for attempt in range(attempts):
    if Bash(adb_cmd):
      return True
  raise Exception('ADB command failed')

def ADBKill(serial, process):
  """ Kill a process running on an Android device.
  
  serial: string indicating the serial number of the target device
  process: string indicating the name of the process to kill
  """ 
  cmd = '%s -s %s shell ps | grep %s' % (PATH_TO_ADB, serial, process)
  stdout = BashGet(cmd)
  if stdout != '':
    pid = shlex.split(stdout)[1]
    kill_cmd = ['shell', 'kill', pid]
    RunADB(serial, kill_cmd)

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
  output = BashGet('%s devices' % PATH_TO_ADB, echo=True)
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
    name_line = BashGet(
        '%s -s %s shell cat /system/build.prop | grep "ro.product.device="' % (
            PATH_TO_ADB, id),
        echo=True)
    print name_line
    name = name_line.split('=')[-1].rstrip()
    # Just return the first attached device of the requested model.
    if device_name in name:
      return id
  raise Exception('No %s device attached!' % device_name)

class _WatchLog(threading.Thread):
  """ Run WatchLog in a new thread to record the logcat output from SkiaAndroid.
  Returns iff a 'SKIA_RETURN_CODE' appears in the log, setting the return code
  appropriately.  Note that this will not terminate if the SkiaAndroid process
  does not finish normally, so we need to periodically check that the process is
  still running and terminate this thread if the process has died without
  printing a 'SKIA_RETURN_CODE'. """
  def __init__(self, serial, logfile=None):
    threading.Thread.__init__(self)
    self.retcode = SKIA_RUNNING
    self.serial = serial
    self._stopped = False
    self._logfile = logfile

  def stop(self):
    self._stopped = True
    self._logger.terminate()

  def run(self):
    self._logger = BashAsync('%s -s %s logcat' % (
        PATH_TO_ADB, self.serial), echo=False)
    while not self._stopped:
      line = self._logger.stdout.readline()
      if line != '':
        if self._logfile:
          self._logfile.write(line)
        print line.rstrip('\r\n')
        if 'SKIA_RETURN_CODE' in line:
          self.retcode = shlex.split(line)[-1]
          self.stop()
          return

def Install(serial):
  try:
    RunADB(serial, ['uninstall', 'com.skia'])
  except:
    pass
  RunADB(serial, ['install', PATH_TO_APK])

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

  RunADB(serial, ['logcat', '-c'])

  cmd_line = binary_name
  for arg in arguments:
    cmd_line = '%s %s' % (cmd_line, arg)
  cmd_line = '"%s"' % cmd_line
  RunADB(serial, ['shell', 'am', 'broadcast',
                  '-a', 'com.skia.intent.action.LAUNCH_SKIA',
                  '-n', 'com.skia/.SkiaReceiver',
                  '-e', 'args'] + shlex.split(cmd_line))
  logger = _WatchLog(serial, logfile=logfile)
  logger.start()
  while logger.isAlive() and logger.retcode == SKIA_RUNNING:
    time.sleep(PROCESS_MONITOR_INTERVAL)
    # adb does not always return in a timely fashion.  Don't wait for it.
    monitor = BashGetTimeout(
        '%s -s %s shell ps | grep skia_native' % (PATH_TO_ADB, serial),
        echo=False)
    if not monitor[0]: # adb timed out
      continue
    # No SKIA_RETURN_CODE printed, but the process isn't running
    if monitor[1] == '' and logger.retcode == SKIA_RUNNING:
      logger.stop()
      raise Exception('Skia process died while executing %s' % binary_name)
  if not logger.retcode == '0':
    raise Exception('Failure in %s' % binary_name)
