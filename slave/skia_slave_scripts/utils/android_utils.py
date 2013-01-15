#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains tools used by Android-specific buildbot scripts. """

import os
import re
import shell_utils
import shlex
import time


PATH_TO_ADB = os.path.join('..', 'android', 'bin', 'linux', 'adb')
DEVICE_LOOKUP = {'nexus_s': 'crespo',
                 'xoom': 'stingray',
                 'galaxy_nexus': 'toro',
                 'nexus_4': 'mako',
                 'nexus_7': 'grouper',
                 'nexus_10': 'manta'}
CPU_SCALING_MODES = ['performance', 'interactive']


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


def ADBKill(serial, process):
  """ Kill a process running on an Android device.
  
  serial: string indicating the serial number of the target device
  process: string indicating the name of the process to kill
  """ 
  try:
    stdout = shell_utils.Bash('%s -s %s shell ps | grep %s' % (
                                  PATH_TO_ADB, serial, process), shell=True)
  except:
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
  for id in device_ids:
    print 'Finding type for id %s' % id
    # Get device name
    name_line = shell_utils.BashRetry(
        '%s -s %s shell cat /system/build.prop | grep "ro.product.device="' % (
            PATH_TO_ADB, id), shell=True, attempts=5)
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
  cpu_dirs = shell_utils.Bash('%s -s %s shell ls /sys/devices/system/cpu' % (
      PATH_TO_ADB, serial), echo=False, shell=True)
  cpu_dirs_list = cpu_dirs.split('\n')
  regex = re.compile('cpu\d')
  for dir in cpu_dirs_list:
    cpu_dir = dir.rstrip()
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


def StopShell(serial):
  """ Halt the Android runtime on the device with the given serial number.
  Blocks until the shell reports that it has stopped.

  serial: string indicating the serial number of the target device.
  """
  ADBShell(serial, ['stop'])
  while IsAndroidShellRunning(serial):
    time.sleep(1)


def StartShell(serial):
  """ Start the Android runtime on the device with the given serial number.
  Blocks until the shell reports that it has started.

  serial: string indicating the serial number of the target device.
  """
  ADBShell(serial, ['start'])
  while not IsAndroidShellRunning(serial):
    time.sleep(1)


def IsSkiaAndroidAppInstalled(serial):
  """ Determine whether the Skia Android app is installed. """
  return bool('com.skia' in ADBShell(serial, ['pm', 'list', 'packages'],
                                     echo=False))


def Install(serial, release_mode=False):
  """ Install an Android app to the device with the given serial number.

  serial: string indicating the serial number of the target device.
  release_mode: bool; whether the app was build in Release mode.
  """
  # The shell must be running to install/uninstall apps
  StartShell(serial)
  # Assuming we're in the 'trunk' directory.
  cmd = [os.path.join(os.pardir, 'android', 'bin', 'android_install_skia'),
         '-f',
         '--install-launcher',
         '-s', serial]
  if release_mode:
    cmd.append('--release')
  shell_utils.Bash(' '.join(cmd), shell=True)
  if not IsSkiaAndroidAppInstalled(serial):
    raise Exception('Failed to install Skia Android app.')


def RunShell(serial, cmd):
  """ Run the given command through skia_launcher on a given device.

  serial: string indicating the serial number of the target device.
  cmd: list of strings; the command to run
  """
  # Ensure that the shell is stopped
  StopShell(serial)
  RunADB(serial, ['logcat', '-c'])
  try:
    ADBShell(serial, ['skia_launcher'] + cmd)
  finally:
    RunADB(serial, ['logcat', '-d', '-v', 'time'])