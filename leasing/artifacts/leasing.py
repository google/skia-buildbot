#!/usr/bin/env python
# Copyright (c) 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""This script should be run on a Swarming bot as part of leasing.skia.org."""


from __future__ import print_function
import argparse
import json
import os
import sys
import time
from urllib.error import HTTPError
from urllib.request import urlopen


POLLING_WAIT_TIME_SECS = 60


def main():
  parser = argparse.ArgumentParser()
  parser.add_argument('-s', '--leasing-server', required=True, type=str,
                      help='The leasing server this script will poll. '
                           'Eg: leasing.skia.org')
  parser.add_argument('-t', '--task-id', required=True, type=str,
                      help='The taskID of this swarming task.')
  parser.add_argument('-o', '--os-type', required=True, type=str,
                      help='The Os Type this script is running on.')
  parser.add_argument('-c', '--debug-command', required=True, type=str,
                      help='The command users can use to run the debug task.')
  parser.add_argument('-r', '--command-relative-dir', required=True, type=str,
                      help='The directory the command should be run in.')
  args = parser.parse_args()

  if args.debug_command:
    print('Files are mapped into: ')
    print(os.getcwd())
    print()
    print('Original command: ')
    print(args.debug_command)
    print()
    print('Dir to run command in: ')
    print(os.path.join(os.getcwd(), args.command_relative_dir))
    print()
  print()
  print('Please cleanup after you are done debugging or when you get the '
        '15 min warning email!')
  sys.stdout.flush()

  while True:
    get_task_status_url = 'http://%s/_/get_task_status?task=%s' % (
        args.leasing_server, args.task_id)
    try:
      resp = urlopen(get_task_status_url)
      output = json.load(resp)
      if output['Expired']:
        break
    except HTTPError as e:
      print('Could not contact the leasing server: %s' % e)

    time.sleep(POLLING_WAIT_TIME_SECS)

  print('The lease time has expired.')
  print('Failing the task so that swarming reboots the host.')
  # Fail the task so that swarming reboots the host. This will force all SSH'ed
  # users to disconnect.
  return 1


if __name__ == '__main__':
  sys.exit(main())
