#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This module contains tools used by the Android buildbot scripts.
"""

import os
import shlex
import subprocess
import time

SUBPROCESS_TIMEOUT = 30.0
PATH_TO_ADB = os.path.join('..', 'android', 'bin', 'linux', 'adb')
PATH_TO_APK = os.path.join('out', 'android', 'bin', 'SkiaAndroid.apk')
DEVICE_LOOKUP = {'nexus_s': 'crespo',
                 'xoom': 'stingray'}

def ConfirmOptionsSet(name_value_dict):
  """Raise an exception if any of the given command-line options were not set.

  @param name_value_dict dictionary mapping option names to option values
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
  return subprocess.call(shlex.split(cmd)) == 0;

def BashGet(cmd, echo=True):
  """ Run 'cmd' in a shell and return the exit code. (Blocking) """
  if echo:
    print(cmd)
  return subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True).communicate()[0]

def BashGetTimeout(cmd, echo=True, timeout=SUBPROCESS_TIMEOUT):
  """ Run 'cmd' in a shell and return the exit code.  Blocks until the command
  is finished or the timeout expires. """
  proc = BashAsync(cmd, echo=echo)
  t_0 = time.time()
  t_elapsed = 0.0
  while not proc.poll() and t_elapsed < timeout:
    t_elapsed = time.time() - t_0
  return proc.poll(), proc.communicate()[0]

def BashAsync(cmd, echo=True):
  """ Run 'cmd' in a subprocess, returning a handle to that process.
  (Non-blocking) """
  if echo:
    print cmd
  return subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True, stderr=subprocess.STDOUT)

def GetSerial(device_type):
  """ Determine the serial number of the *first* connected device with the
  specified type.  The ordering of 'adb devices' is not documented, and the
  connected devices do not appear to be ordered by serial number.  Therefore,
  we have to assume that, in the case of multiple devices of the same type being
  connected to one host, we cannot predict which device will be chosen. """
  if not device_type in DEVICE_LOOKUP:
    print 'Unknown device: %s!' % device_type
    return None
  device_name = DEVICE_LOOKUP[device_type]
  output = BashGet('%s devices' % PATH_TO_ADB, echo=False)
  lines = output.split('\n')
  # Header line is garbage
  lines.pop(0)
  device_ids = []
  for line in lines:
    if line != '':
      device_ids.append(line.split('\t')[0])
  for id in device_ids:
    # Get device name
    name_line = BashGet(
        '%s -s %s shell cat /system/build.prop | grep "ro.product.device="' % (
            PATH_TO_ADB, id),
        echo=False)
    name = name_line.split('=')[-1].rstrip()
    # Just return the first attached device of the requested model.
    if device_name in name:
      return id
  print 'No %s device attached!' % device_name
  return None