#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" This script runs a Skia binary inside of an Android APK.

To run:
  python /path/to/slave/scripts/run_android --binary_name $BINARY --args $ARGS

Where BINARY is the name of the Skia binary to run, eg. gm or tests
And ARGS are any command line arguments to pass to that binary.

For example:
  python /path/to/slave/scripts/run_android -- binary_name gm --args --nopdf

"""

import optparse
import os
import shlex
import subprocess
import sys
import threading
import time

PROCESS_MONITOR_INTERVAL = 5.0 # Seconds
SUBPROCESS_TIMEOUT = 30.0
PATH_TO_ADB = os.path.join('..', 'android', 'bin', 'linux', 'adb')
PATH_TO_APK = os.path.join('out', 'android', 'bin', 'SkiaAndroid.apk')
SKIA_RUNNING = 'running'
DEVICE_LOOKUP = {'nexus_s': 'crespo',
                 'xoom': 'stingray'}

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

class WatchLog(threading.Thread):
  """ Run WatchLog in a new thread to record the logcat output from SkiaAndroid. 
  Returns iff a 'SKIA_RETURN_CODE' appears in the log, setting the return code
  appropriately.  Note that this will not terminate if the SkiaAndroid process
  does not finish normally, so we need to periodically check that the process is
  still running and terminate this thread if the process has died without
  printing a 'SKIA_RETURN_CODE'. """
  def __init__(self, serial):
    threading.Thread.__init__(self)
    self.retcode = SKIA_RUNNING
    self.serial = serial

  def run(self):
    logger = BashAsync('%s -s %s logcat' % (PATH_TO_ADB, self.serial), echo=False)
    while True:
      line = logger.stdout.readline()
      if line != '':
        print line.rstrip('\r\n')
        if 'SKIA_RETURN_CODE' in line:
          self.retcode = shlex.split(line)[-1]
          logger.terminate()
          return

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
    name_line = BashGet('%s -s %s shell cat /system/build.prop | grep "ro.product.device="' % (PATH_TO_ADB, id), echo=False)
    name = name_line.split('=')[-1].rstrip()
    # Just return the first attached device of the requested model.
    if device_name in name:
      return id
  print 'No %s device attached!' % device_name
  return None

def Run(errors, device, binary_name, arguments=''):
  """ Run 'binary_name' with 'arguments'.  This function sets and runs the Skia
  APK on a connected device.  We launch WatchLog in a new thread and then keep
  polling the device to make sure that the process is still running.  We are
  done when either:
  
  1. WatchLog sets a value in 'retcode'
  2. WatchLog has not set a value in 'retcode' and the Skia process has died.
  
  We then return success or failure. """
  serial = GetSerial(device)
  if not serial:
    errors.append('Could not find device!')
    return False
  if not (Bash('%s -s %s root' % (PATH_TO_ADB, serial)) and \
          Bash('%s -s %s remount' % (PATH_TO_ADB, serial))):
    errors.append('Unable to root and remount device.')
    return False
  Bash('%s -s %s uninstall com.skia' % (PATH_TO_ADB, serial))
  if not (Bash('%s -s %s install %s' % (PATH_TO_ADB, serial, PATH_TO_APK)) and \
          Bash('%s -s %s logcat -c' % (PATH_TO_ADB, serial))):
    errors.append('Could not install APK to device.')
    return False
  if not Bash('%s -s %s shell am broadcast -a com.skia.intent.action.LAUNCH_SKIA -n com.skia/.SkiaReceiver -e args "%s %s"' % (PATH_TO_ADB, serial, binary_name, arguments)):
    return False
  logger = WatchLog(serial)
  logger.start()
  while logger.isAlive() and logger.retcode == SKIA_RUNNING:
    time.sleep(PROCESS_MONITOR_INTERVAL)
    # adb does not always return in a timely fashion.  Don't wait for it.
    monitor = BashGetTimeout('%s -s %s shell ps | grep skia_native' % (PATH_TO_ADB, serial), echo=False)
    if not monitor[0]: # adb timed out
      continue
    # No SKIA_RETURN_CODE printed, but the process isn't running
    if monitor[1] == '' and logger.retcode == '':
      logger.retcode = -1
      errors.append('Skia process died while executing %s' % binary_name)
      break
  if logger.retcode == '0':
    return True
  else:
    errors.append('Failure in %s' % binary_name)
    return False

""" TODO(borenet): This is copy/pasted from merge_into_svn.py.  We should
refactor to share this code. """
def ConfirmOptionsSet(name_value_dict):
  """Raise an exception if any of the given command-line options were not set.

  @param name_value_dict dictionary mapping option names to option values
  """
  for (name, value) in name_value_dict.iteritems():
    if value is None:
      raise Exception('missing command-line option %s; rerun with --help' %
                      name)

def main(argv):
  """ Verify that the command-line options are set and then call Run() to
  install and run the APK. """
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '--binary_name',
      help='name of the Skia binary to launch')
  option_parser.add_option(
      '--device',
      help='type of device on which to run the binary')
  option_parser.add_option(
      '--args',
      help='arguments to pass to the Skia binary')
  (options, args) = option_parser.parse_args()
  if len(args) != 0:
    raise Exception('bogus command-line argument; rerun with --help')
  ConfirmOptionsSet({'--binary_name': options.binary_name})
  ConfirmOptionsSet({'--device': options.device})
  errors = []
  success = Run(errors, options.device, options.binary_name, arguments=options.args)
  if errors or not success:
    for e in errors:
      print 'ERROR: %s' % e
    return 1
  return 0

if '__main__' == __name__:
  sys.exit(main(None))