#!/usr/bin/env python
#
# Copyright 2018 Google LLC
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from __future__ import print_function
import os
import shutil
import subprocess
import sys


swarming_task_id = os.environ['SWARMING_TASK_ID']
kitchen = os.path.join(os.getcwd(), 'kitchen')
logdog_url = 'logdog://logs.chromium.org/%s/%s/+/annotations' % (
    sys.argv[4], swarming_task_id)
temp_dir = 'tmp'
if os.path.isdir('/dev/shm'):
  # Recent Linux provides /dev/shm, which is backed by a tmpfs. Since persistent
  # disks seem to be very slow on GCE, use a subdir of /dev/shm as the temp-dir
  # instead.
  temp_dir = os.path.join('/dev/shm', swarming_task_id)

cmd = [
  kitchen, 'cook',
    '-checkout-dir', 'recipe_bundle',
    '-mode', 'swarming',
    '-luci-system-account', 'system',
    '-cache-dir', 'cache',
    '-temp-dir', temp_dir,
    '-known-gerrit-host', 'android.googlesource.com',
    '-known-gerrit-host', 'boringssl.googlesource.com',
    '-known-gerrit-host', 'chromium.googlesource.com',
    '-known-gerrit-host', 'dart.googlesource.com',
    '-known-gerrit-host', 'fuchsia.googlesource.com',
    '-known-gerrit-host', 'go.googlesource.com',
    '-known-gerrit-host', 'llvm.googlesource.com',
    '-known-gerrit-host', 'skia.googlesource.com',
    '-known-gerrit-host', 'webrtc.googlesource.com',
    '-recipe', sys.argv[2],
    '-properties', sys.argv[3],
    '-logdog-annotation-url', logdog_url,
]
print('running command: %s' % ' '.join(cmd))
subprocess.check_call(cmd)

print('cleaning up %s...' % temp_dir)
shutil.rmtree(temp_dir, ignore_errors=True)

print('finished run_recipe for %s' % sys.argv[2])
