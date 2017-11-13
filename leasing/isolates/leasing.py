#!/usr/bin/env python
# Copyright (c) 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This script should be run on a Swarming bot as part of leasing.skia.org."""

import argparse
import json
import os
import sys
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
  args = parser.parse_args()

  while True:
    get_task_status_url = '%s/_/get_task_status?task=%s' % (
        args.leasing_server, args.task_id)
    resp = urllib2.urlopen(get_task_status_url)

    output = json.load(resp)
    print 'Response from %s is: %s' % (get_task_status_url, output)
    sys.stdout.flush()

    if output['Expired']:
      break
    time.sleep(POLLING_WAIT_TIME_SECS)

  # Fail the task so that swarming reboots the bot. This will force all SSH'ed
  # users to disconnect.
  return 1


if __name__ == '__main__':
  sys.exit(main())
