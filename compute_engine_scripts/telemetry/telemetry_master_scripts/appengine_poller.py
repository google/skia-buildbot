#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that polls the skia-telemetry AppEngine WebApp.

Admin and Lua tasks are polled by this module. All new tasks are then triggered.
This module also periodically updates the Telemetry Information after
UPDATE_INFO_AFTER_SECS have elapsed.
"""


import json
import os
import subprocess
import sys
import tempfile
import time
import traceback
import urllib

import appengine_constants


SLEEP_BETWEEN_POLLS_SECS = 30

UPDATE_INFO_AFTER_SECS = 7200

# The following dictionaries ensure that tasks which are being currently
# processed are not triggered again.
ADMIN_ENCOUNTERED_KEYS = {}
CHROMIUM_BUILD_ENCOUNTERED_KEYS = {}
CHROMIUM_TRY_ENCOUNTERED_KEYS = {}
LUA_ENCOUNTERED_KEYS = {}
TELEMETRY_ENCOUNTERED_KEYS = {}


def process_admin_task(task):
  # Extract required parameters.
  task_key = task['key']
  if task_key in ADMIN_ENCOUNTERED_KEYS:
    print '%s is already being processed' % task_key
    return
  ADMIN_ENCOUNTERED_KEYS[task_key] = 1

  task_name = task['task_name']
  username = task['username']
  pagesets_type = task['pagesets_type']

  log_file = os.path.join(tempfile.gettempdir(), '%s-%s.output' % (
      username, task_key))
  print 'Admin output will be available in %s' % log_file

  cmd = ''
  if task_name == appengine_constants.PAGESETS_ADMIN_TASK_NAME:
    cmd = 'bash vm_create_pagesets_on_slaves.sh %s %s %s' % (
        username, task_key, pagesets_type)
  elif task_name == appengine_constants.WEBPAGE_ARCHIVES_ADMIN_TASK_NAME:
    chromium_build_dir = get_chromium_build_dir(task['chromium_rev'],
                                                task['skia_rev'])
    cmd = 'bash vm_capture_archives_on_slaves.sh %s %s %s %s' % (
        username, task_key, pagesets_type, chromium_build_dir)
  elif task_name == appengine_constants.PDFVIEWER_ADMIN_TASK_NAME:
    run_id = '%s-%s' % (task['username'].split('@')[0], time.time())
    cmd = 'bash vm_run_pdf_viewer_on_slaves.sh %s %s %s %s %s' % (
        username, run_id, pagesets_type, task_key, log_file)
  subprocess.Popen(cmd.split(), stdout=open(log_file, 'w'),
                   stderr=open(log_file, 'w'))


def process_chromium_build_task(task):
  # Extract required parameters.
  task_key = task['key']
  if task_key in CHROMIUM_BUILD_ENCOUNTERED_KEYS:
    print '%s is already being processed' % task_key
    return
  CHROMIUM_BUILD_ENCOUNTERED_KEYS[task_key] = 1

  chromium_rev = task['chromium_rev']
  skia_rev = task['skia_rev']
  username = task['username']

  log_file = os.path.join(tempfile.gettempdir(), '%s-%s.output' % (
      username, task_key))
  print 'Chromium build output will be available in %s' % log_file
  cmd = 'bash vm_build_chromium.sh %s %s %s %s %s' % (
      chromium_rev, skia_rev, username, task_key, log_file)
  subprocess.Popen(cmd.split(), stdout=open(log_file, 'w'),
                   stderr=open(log_file, 'w'))


def process_chromium_try_task(task):
  # Extract required parameters.
  task_key = task['key']
  if task_key in CHROMIUM_TRY_ENCOUNTERED_KEYS:
    print '%s is already being processed' % task_key
    return
  CHROMIUM_TRY_ENCOUNTERED_KEYS[task_key] = 1

  username = task['username']
  benchmark_name = task['benchmark_name']
  benchmark_arguments = task['benchmark_arguments']
  # Escape any quotes in benchmark arguments.
  benchmark_arguments = benchmark_arguments.replace('"', r'\"')
  patch_type = task['patch_type']
  variance_threshold = task['variance_threshold']
  discard_outliers = task['discard_outliers']
  # Copy the patch to a local file.
  run_id = '%s-%s' % (username.split('@')[0], time.time())
  patch_txt = task['patch'].replace('\r\n', '\n')
  # Add an extra newline at the end because git sometimes rejects patches due to
  # missing newlines.
  patch_txt += '\n'
  patch_file = os.path.join(tempfile.gettempdir(),
                            '%s.patch' % run_id)
  f = open(patch_file, 'w')
  f.write(patch_txt)
  f.close()

  log_file = os.path.join(tempfile.gettempdir(), '%s.output' % run_id)
  print 'Chromium try output will be available in %s' % log_file
  cmd = ('bash vm_run_chromium_try.sh -p %(patch_file)s -t %(patch_type)s '
         '-r %(run_id)s -v %(variance_threshold)s -o %(discard_outliers)s '
         '-b %(benchmark_name)s -a %(benchmark_arguments)s -e %(username)s '
         '-i %(task_key)s -l %(log_file)s' % {
             'patch_file': patch_file,
             'patch_type': patch_type,
             'run_id': run_id,
             'variance_threshold': variance_threshold,
             'discard_outliers': discard_outliers,
             'benchmark_name': benchmark_name,
             'benchmark_arguments': benchmark_arguments,
             'username': username,
             'task_key': task_key,
             'log_file': log_file})
  subprocess.Popen(cmd.split(), stdout=open(log_file, 'w'),
                   stderr=open(log_file, 'w'))


def process_lua_task(task):
  task_key = task['key']
  pagesets_type = task['pagesets_type']
  if task_key in LUA_ENCOUNTERED_KEYS:
    print '%s is already being processed' % task_key
    return
  LUA_ENCOUNTERED_KEYS[task_key] = 1
  chromium_build_dir = get_chromium_build_dir(task['chromium_rev'],
                                              task['skia_rev'])
  # Create a run id.
  run_id = '%s-%s' % (task['username'].split('@')[0], time.time())
  lua_file = os.path.join(tempfile.gettempdir(), '%s.lua' % run_id)
  f = open(lua_file, 'w')
  f.write(task['lua_script'])
  f.close()

  # Now call the vm_run_lua_on_slaves.sh script.
  log_file = os.path.join(tempfile.gettempdir(), '%s.output' % run_id)
  cmd = 'bash vm_run_lua_on_slaves.sh %s %s %s %s %s %s' % (
      lua_file, run_id, pagesets_type, chromium_build_dir, task['username'],
      task_key)
  print 'Lua output will be available in %s' % log_file
  subprocess.Popen(cmd.split(), stdout=open(log_file, 'w'),
                   stderr=open(log_file, 'w'))


def process_telemetry_task(task):
  task_key = task['key']
  if task_key in TELEMETRY_ENCOUNTERED_KEYS:
    print '%s is already being processed' % task_key
    return
  TELEMETRY_ENCOUNTERED_KEYS[task_key] = 1
  benchmark_name = task['benchmark_name']
  benchmark_arguments = task['benchmark_arguments']
  # Escape any quotes in benchmark arguments.
  benchmark_arguments = benchmark_arguments.replace('"', r'\"')
  pagesets_type = task['pagesets_type']
  chromium_build_dir = get_chromium_build_dir(task['chromium_rev'],
                                              task['skia_rev'])
  username = task['username']
  # Create a run id.
  run_id = '%s-%s' % (username.split('@')[0], time.time())

  # Now call the vm_run_telemetry_on_slaves.sh script.
  log_file = os.path.join(tempfile.gettempdir(), '%s.output' % run_id)
  cmd = [
      'bash',
      'vm_run_telemetry_on_slaves.sh',
      benchmark_name,
      benchmark_arguments,
      pagesets_type,
      chromium_build_dir,
      run_id,
      username,
      str(task_key),
      log_file
  ]
  if task.get('whitelist_file'):
    whitelist_file = os.path.join(tempfile.gettempdir(),
                                  '%s.whitelist' % run_id)
    f = open(whitelist_file, 'w')
    f.write(task['whitelist_file'])
    f.close()
    cmd.append(whitelist_file)
  print 'Telemetry output will be available in %s' % log_file
  subprocess.Popen(cmd, stdout=open(log_file, 'w'),
                   stderr=open(log_file, 'w'))


def get_chromium_build_dir(chromium_rev, skia_rev):
  """Construct the chromium build dir from chromium and skia revs."""
  return '%s-%s' % (chromium_rev[0:7], skia_rev[0:7])

TASK_TYPE_TO_PROCESSING_METHOD = {
    appengine_constants.ADMIN_TASK_NAME: process_admin_task,
    appengine_constants.CHROMIUM_BUILD_TASK_NAME: process_chromium_build_task,
    appengine_constants.CHROMIUM_TRY_TASK_NAME: process_chromium_try_task,
    appengine_constants.LUA_TASK_NAME: process_lua_task,
    appengine_constants.TELEMETRY_TASK_NAME: process_telemetry_task,
}


class Poller(object):

  def Poll(self):
    info_updated_on = 0
    while True:
      try:
        if (time.time() - info_updated_on) >= UPDATE_INFO_AFTER_SECS:
          log_file = os.path.join(tempfile.gettempdir(), 'update-info.output')
          for cmd in ('python update_appengine_info.py',
                      'bash vm_recover_slaves_from_crashes.sh'):
            script_name = cmd.split()[1]
            log_file = os.path.join(tempfile.gettempdir(), script_name)
            print '%s output will be available in %s' % (script_name, log_file)
            subprocess.Popen(cmd.split(), stdout=open(log_file, 'w'),
                             stderr=open(log_file, 'w'))
          info_updated_on = time.time()

        # pylint: disable=C0301
        oldest_pending_task_page = urllib.urlopen(
            appengine_constants.SKIA_TELEMETRY_WEBAPP +
            appengine_constants.GET_OLDEST_PENDING_TASK_SUBPATH)
        oldest_pending_task = json.loads(
            oldest_pending_task_page.read().replace('\r\n', '\\r\\n'))
        if oldest_pending_task:
          task_type = oldest_pending_task.keys()[0]
          processing_method = TASK_TYPE_TO_PROCESSING_METHOD[task_type]
          processing_method(oldest_pending_task[task_type])

        print 'Sleeping %s secs' % SLEEP_BETWEEN_POLLS_SECS
        time.sleep(SLEEP_BETWEEN_POLLS_SECS)
      except Exception:
        # The poller should never crash, output the exception and move on.
        print traceback.format_exc()
        continue


if '__main__' == __name__:
  sys.exit(Poller().Poll())
