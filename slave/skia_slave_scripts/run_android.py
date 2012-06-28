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
SKIA_RUNNING = 'running'

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

""" Run 'cmd' in a shell and return the exit code.  Blocks until the command is
finished or the timeout expires. """
def BashGetTimeout(cmd, echo=True, timeout=SUBPROCESS_TIMEOUT):
  proc = BashAsync(cmd, echo=echo)
  t_0 = time.time()
  t_elapsed = 0.0
  while not proc.poll() and t_elapsed < timeout:
    t_elapsed = time.time() - t_0
  return proc.poll(), proc.communicate()[0]

""" Run 'cmd' in a subprocess, returning a handle to that process.
(Non-blocking) """
def BashAsync(cmd, echo=True):
  if echo:
    print cmd
  return subprocess.Popen(cmd, stdout=subprocess.PIPE, shell=True, stderr=subprocess.STDOUT)

""" Run WatchLog in a new thread to record the logcat output from SkiaAndroid. 
Returns iff a 'SKIA_RETURN_CODE' appears in the log, setting the return code
appropriately.  Note that this will not terminate if the SkiaAndroid process
does not finish normally, so we need to periodically check that the process is
still running and terminate this thread if the process has died without printing
a 'SKIA_RETURN_CODE'. """
class WatchLog(threading.Thread):
  def __init__(self):
    threading.Thread.__init__(self)
    self.retcode = SKIA_RUNNING

  def run(self):
    logger = BashAsync('%s logcat' % PATH_TO_ADB, echo=False)
    while True:
      line = logger.stdout.readline()
      if line != '':
        print line.rstrip('\r\n')
        if 'SKIA_RETURN_CODE' in line:
          self.retcode = shlex.split(line)[-1]
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
  if not (Bash('%s root' % PATH_TO_ADB) and \
          Bash('%s remount' % PATH_TO_ADB)):
    errors.append('Unable to root and remount device.')
    return False
  Bash('%s uninstall com.skia' % PATH_TO_ADB)
  path_to_apk = os.path.join('out', 'android', 'bin', 'SkiaAndroid.apk')
  if not (Bash('%s install %s' % (PATH_TO_ADB, path_to_apk)) and \
          Bash('%s logcat -c' % PATH_TO_ADB)):
    errors.append('Could not install APK to device.')
    return False
  if not Bash('%s shell am broadcast -a com.skia.intent.action.LAUNCH_SKIA -n com.skia/.SkiaReceiver -e args "%s %s"' % (PATH_TO_ADB, binary_name, arguments)):
    return False
  logger = WatchLog()
  logger.start()
  while logger.isAlive() and logger.retcode == SKIA_RUNNING:
    time.sleep(PROCESS_MONITOR_INTERVAL)
    # adb does not always return in a timely fashion.  Don't wait for it.
    monitor = BashGetTimeout('%s shell ps | grep skia_native' % PATH_TO_ADB, echo=False)
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