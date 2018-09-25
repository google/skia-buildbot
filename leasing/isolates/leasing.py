#!/usr/bin/env python
# Copyright (c) 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This script should be run on a Swarming bot as part of leasing.skia.org."""

import argparse
import json
import os
import socket
import subprocess
import sys
import tempfile
import time
import urllib2


POLLING_WAIT_TIME_SECS = 60


def main():
  parser = argparse.ArgumentParser()
  parser.add_argument('-s', '--leasing-server', required=True, type=str,
                      help='The leasing server this script will poll.')
  parser.add_argument('-t', '--task-id', required=True, type=str,
                      help='The taskID of this swarming task.')
  parser.add_argument('-o', '--os-type', required=True, type=str,
                      help='The Os Type this script is running on.')
  parser.add_argument('-c', '--debug-command', required=True, type=str,
                      help='The command users can use to run the debug task.')
  parser.add_argument('-r', '--command-relative-dir', required=True, type=str,
                      help='The directory the command should be run in.')
  parser.add_argument('-g', '--skiaserve-gs-path', required=False, type=str,
                      default=None,
                      help='GS location of skiaserve binary to use.')
  args = parser.parse_args()

  if args.debug_command:
    print 'Files are mapped into: '
    print os.getcwd()
    print
    print 'Original command: '
    print args.debug_command
    print
    print 'Dir to run command in: '
    print os.path.join(os.getcwd(), args.command_relative_dir)
    print

  try:
    if args.skiaserve_gs_path:
      gsutil_binary = os.path.join('cipd_bin_packages', 'gsutil')
      skiaserve_device_location = '/data/local/tmp/skiaserve'
      # Copy skiaserver binary from Google storage to the device.
      _, tmpFile = tempfile.mkstemp()
      try:
        print 'Copying %s locally to %s' % (args.skiaserve_gs_path, tmpFile)
        subprocess.check_call(
            [gsutil_binary, 'cp', args.skiaserve_gs_path, tmpFile])
        print 'Copying %s to device %s' % (tmpFile, skiaserve_device_location)
        subprocess.check_call(
            ['adb', 'push', tmpFile, skiaserve_device_location])
      finally:
        os.remove(tmpFile)
      # Start the debugger and setup adb port forwarding from host to device.
      print 'Bringing up the debugger'
      subprocess.check_call(
          ['adb', 'shell', 'chmod', '777', skiaserve_device_location])
      proc = subprocess.Popen(['adb', 'shell', skiaserve_device_location])
      print 'Running adb forward tcp:8888 tcp:8888'
      subprocess.check_call(['adb', 'forward', 'tcp:8888', 'tcp:8888'])
      print

    print
    print ('Please cleanup after you are done debugging or when you get the '
           '15 min warning email!')
    sys.stdout.flush()

    while True:
      get_task_status_url = '%s/_/get_task_status?task=%s' % (
          args.leasing_server, args.task_id)
      try:
        resp = urllib2.urlopen(get_task_status_url)
        output = json.load(resp)
        if output['Expired']:
          break
      except urllib2.HTTPError as e:
        print 'Could not contact the leasing server: %s' % e

      time.sleep(POLLING_WAIT_TIME_SECS)

    print 'The lease time has expired.'

  finally:
    # Cleanup everything that was setup for the debugger.
    if args.skiaserve_gs_path:
      # Stop the debugger.
      proc.terminate()
      # Cleanup the debugger binary.
      subprocess.check_call(['adb', 'shell', 'rm', skiaserve_device_location])
      # Remove adb port forwarding.
      subprocess.check_call(['adb', 'forward', '--remove', 'tcp:8888'])

  print 'Failing the task so that swarming reboots the host.'
  # Fail the task so that swarming reboots the host. This will force all SSH'ed
  # users to disconnect.
  return 1


if __name__ == '__main__':
  sys.exit(main())
