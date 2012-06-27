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

""" Run 'cmd' in a shell and return True iff the 'cmd' succeeded. (Blocking) """
def Bash(cmd, echo=True):
  if echo:
    print cmd
  return subprocess.call(shlex.split(cmd)) == 0;

""" Run 'cmd' in a shell and return the exit code. (Blocking) """
def BashGet(cmd, echo=True):
  if echo:
    print(cmd)
  return subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True).communicate()[0]

""" Run 'cmd' in a subprocess, returning a handle to that process.
(Non-blocking) """
def BashAsync(cmd, echo=True):
  if echo:
    print cmd
  return subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True, stderr=subprocess.STDOUT)

""" Wrapper class to hold the return code of the Skia program. """
class ReturnCode(object):
  def __init__(self, retcode):
    self._retcode = retcode
  
  def get(self):
    return self._retcode
  
  def set(self, retcode):
    self._retcode = retcode

path_to_adb = os.path.join('..', 'android', 'bin', 'linux', 'adb')

""" Run WatchLog in a new thread to record the logcat output from SkiaAndroid. 
Returns iff a 'SKIA_RETURN_CODE' appears in the log, setting the return code
appropriately.  Note that this will not terminate if the SkiaAndroid process
does not finish normally, so we need to periodically check that the process is
still running and terminate this thread if the process has died without printing
a 'SKIA_RETURN_CODE'. """
def WatchLog(retcode):
  logger = BashAsync('%s logcat' % path_to_adb, echo=False)
  while True:
    line = logger.stdout.readline()
    if line != '':
      print line.rstrip('\r\n')
      if 'SKIA_RETURN_CODE' in line:
        retcode.set(shlex.split(line)[-1])
        logger.terminate()
        return

""" Run 'binary_name' with 'arguments'.  This function sets and runs the Skia
APK on a connected device.  We launch WatchLog in a new thread and then keep
polling the device to make sure that the process is still running.  We are done
when either:

1. WatchLog sets a value in 'retcode'
2. WatchLog has not set a value in 'retcode' and the Skia process has died.

We then return success or failure. """
def Run(errors, binary_name, arguments=''):
  if not (Bash('%s root' % path_to_adb) and \
          Bash('%s remount' % path_to_adb)):
    errors.append('Unable to root and remount device.')
    return False
  Bash('%s uninstall com.skia' % path_to_adb)
  path_to_apk = os.path.join('out', 'android', 'bin', 'SkiaAndroid.apk')
  if not (Bash('%s install %s' % (path_to_adb, path_to_apk)) and \
          Bash('%s logcat -c' % path_to_adb)):
    errors.append('Could not install APK to device.')
    return False
  retcode = ReturnCode('')
  logger = threading.Thread(target=WatchLog, args=(retcode,));
  logger.start()
  if not Bash('%s shell am broadcast -a com.skia.intent.action.LAUNCH_SKIA -n com.skia/.SkiaReceiver -e args "%s %s"' % (path_to_adb, binary_name, arguments)):
    return False
  process_monitor_interval = 5.0 # Seconds
  while logger.isAlive():
    time.sleep(process_monitor_interval)
    still_running = BashGet('%s shell ps | grep skia_native' % path_to_adb, echo=False)
    if still_running == '' and retcode.get() == '':
      retcode.set('-1')
      errors.append('Skia process died while executing %s' % binary_name)
      break
  if retcode.get() == '0':
    return True
  else:
    errors.append('Failure in %s' % binary_name)
    return False

""" Assert that the given command line option is set.
TODO(borenet): This is copy/pasted from merge_into_svn.py.  We should refactor
to share this code. """
def ConfirmOptionsSet(name_value_dict):
  """Raise an exception if any of the given command-line options were not set.

  @param name_value_dict dictionary mapping option names to option values
  """
  for (name, value) in name_value_dict.iteritems():
    if value is None:
      raise Exception('missing command-line option %s; rerun with --help' %
                      name)

""" Verify that the command-line options are set and then call Run() to install
and run the APK. """
def main(argv):
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '--binary_name',
      help='name of the Skia binary to launch')
  option_parser.add_option(
      '--args',
      help='arguments to pass to the Skia binary')
  (options, args) = option_parser.parse_args()
  if len(args) != 0:
    raise Exception('bogus command-line argument; rerun with --help')
  ConfirmOptionsSet({'--binary_name': options.binary_name})
  errors = []
  success = Run(errors, binary_name=options.binary_name, arguments=options.args)
  if errors or not success:
    for e in errors:
      print 'ERROR: %s' % e
    return 1
  return 0

if '__main__' == __name__:
  sys.exit(main(None))