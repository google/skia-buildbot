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
import shlex
import skia_slave_utils
import sys
import threading

PROCESS_MONITOR_INTERVAL = 5.0 # Seconds
SKIA_RUNNING = 'running'

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
    logger = skia_slave_utils.BashAsync('%s -s %s logcat' % (
        skia_slave_utils.PATH_TO_ADB, self.serial), echo=False)
    while True:
      line = logger.stdout.readline()
      if line != '':
        print line.rstrip('\r\n')
        if 'SKIA_RETURN_CODE' in line:
          self.retcode = shlex.split(line)[-1]
          logger.terminate()
          return

<<<<<<< .mine
=======
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

>>>>>>> .r4481
def Run(errors, device, binary_name, arguments=''):
  """ Run 'binary_name' with 'arguments'.  This function sets and runs the Skia
  APK on a connected device.  We launch WatchLog in a new thread and then keep
  polling the device to make sure that the process is still running.  We are
  done when either:
  
  1. WatchLog sets a value in 'retcode'
  2. WatchLog has not set a value in 'retcode' and the Skia process has died.
  
  We then return success or failure. """
  serial = skia_slave_utils.GetSerial(device)
  if not serial:
    errors.append('Could not find device!')
    return False
  # TODO(borenet): Do we still need to root and remount?
  if not (skia_slave_utils.Bash('%s -s %s root' % (
              skia_slave_utils.PATH_TO_ADB, serial)) and \
          skia_slave_utils.Bash('%s -s %s remount' % (
              skia_slave_utils.PATH_TO_ADB, serial))):
    errors.append('Unable to root and remount device.')
    return False
  skia_slave_utils.Bash('%s -s %s uninstall com.skia' % (
      skia_slave_utils.PATH_TO_ADB, serial))
  if not (skia_slave_utils.Bash('%s -s %s install %s' % (
              skia_slave_utils.PATH_TO_ADB, serial,
              skia_slave_utils.PATH_TO_APK)) and \
          skia_slave_utils.Bash('%s -s %s logcat -c' % (
              skia_slave_utils.PATH_TO_ADB, serial))):
    errors.append('Could not install APK to device.')
    return False
  if not skia_slave_utils.Bash(
      '%s -s %s shell am broadcast -a com.skia.intent.action.LAUNCH_SKIA -n '
      'com.skia/.SkiaReceiver -e args "%s %s"' % (
          skia_slave_utils.PATH_TO_ADB, serial, binary_name, arguments)):
    return False
  logger = WatchLog(serial)
  logger.start()
  while logger.isAlive() and logger.retcode == SKIA_RUNNING:
    time.sleep(PROCESS_MONITOR_INTERVAL)
    # adb does not always return in a timely fashion.  Don't wait for it.
    monitor = skia_slave_utils.BashGetTimeout(
        '%s -s %s shell ps | grep skia_native' % (
            skia_slave_utils.PATH_TO_ADB, serial),
        echo=False)
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
  skia_slave_utils.ConfirmOptionsSet({
      '--binary_name': options.binary_name,
      '--device': options.device,
      })
  errors = []
  success = Run(errors, options.device, options.binary_name,
      arguments=options.args)
  if errors or not success:
    for e in errors:
      print 'ERROR: %s' % e
    return 1
  return 0

if '__main__' == __name__:
  sys.exit(main(None))