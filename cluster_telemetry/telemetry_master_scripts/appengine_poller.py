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
SKIA_TRY_ENCOUNTERED_KEYS = {}


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


def fix_and_write_patch(patch, run_id):
  """Modifies the patch for consumption by slaves and writes to local file."""
  # Remove all carriage returns, appengine adds them to blobs.
  patch_txt = patch.replace('\r\n', '\n')
  # Add an extra newline at the end because git sometimes rejects patches due to
  # missing newlines.
  patch_txt += '\n'
  patch_file = os.path.join(tempfile.gettempdir(),
                            '%s.patch' % run_id)
  f = open(patch_file, 'w')
  f.write(patch_txt)
  f.close()
  return patch_file


def process_skia_try_task(task):
  # Extract required parameters.
  task_key = task['key']
  if task_key in SKIA_TRY_ENCOUNTERED_KEYS:
    print '%s is already being processed' % task_key
    return
  SKIA_TRY_ENCOUNTERED_KEYS[task_key] = 1

  username = task['username']
  run_id = '%s-%s' % (username.split('@')[0], time.time())

  patch_file = fix_and_write_patch(task['patch'], run_id)
  pagesets_type = task['pagesets_type']
  chromium_build_dir = get_chromium_build_dir(task['chromium_rev'],
                                              task['skia_rev'])
  render_pictures_args = task['render_pictures_args'].replace('"', r'\"')
  mesa_nopatch_run = task['mesa_nopatch_run']
  mesa_withpatch_run = task['mesa_withpatch_run']

  log_file = os.path.join(tempfile.gettempdir(), '%s.output' % run_id)

  print 'Skia try output will be available in %s' % log_file
  skia_try_cmd = [
      'bash',
      'vm_run_skia_try.sh',
      '-p', str(patch_file),
      '-t', str(pagesets_type),
      '-r', str(run_id),
      '-b', str(chromium_build_dir),
      '-a', str(render_pictures_args),
      '-n', str(mesa_nopatch_run),
      '-w', str(mesa_withpatch_run),
      '-e', str(username),
      '-k', str(task_key),
      '-l', str(log_file)
  ]
  subprocess.Popen(skia_try_cmd, stdout=open(log_file, 'w'),
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
  num_repeated_runs = task['num_repeated_runs']
  variance_threshold = task['variance_threshold']
  discard_outliers = task['discard_outliers']
  pageset_type = task['pageset_type']
  # Copy the patch to a local file.
  run_id = '%s-%s' % (username.split('@')[0], time.time())
  chromium_patch_file = fix_and_write_patch(task['chromium_patch'],
                                            run_id + '.chromium')
  blink_patch_file = fix_and_write_patch(task['blink_patch'],
                                         run_id + '.blink')
  skia_patch_file = fix_and_write_patch(task['skia_patch'],
                                        run_id + '.skia')
  log_file = os.path.join(tempfile.gettempdir(), '%s.output' % run_id)

  print 'Chromium try output will be available in %s' % log_file

  if benchmark_name == 'pixeldiffs':
    cmd = [
        'bash',
        'vm_run_pixeldiffs_try.sh',
        '-p', str(chromium_patch_file),
        '-t', str(blink_patch_file),
        '-s', str(skia_patch_file),
        '-r', str(run_id),
        '-e', str(username),
        '-i', str(task_key),
        '-l', str(log_file),
    ]
  else:
    cmd = [
        'bash',
        'vm_run_chromium_try.sh',
        '-p', str(chromium_patch_file),
        '-t', str(blink_patch_file),
        '-s', str(skia_patch_file),
        '-r', str(run_id),
        '-v', str(variance_threshold),
        '-o', str(discard_outliers),
        '-b', str(benchmark_name),
        '-a', str(benchmark_arguments),
        '-e', str(username),
        '-i', str(task_key),
        '-l', str(log_file),
        '-y', str(pageset_type),
        '-n', str(num_repeated_runs),
    ]
  subprocess.Popen(cmd, stdout=open(log_file, 'w'),
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

  if task.get('lua_aggregator'):
    aggregator_file = os.path.join(tempfile.gettempdir(),
                                   '%s.aggregator' % run_id)
    f = open(aggregator_file, 'w')
    f.write(task['lua_aggregator'])
    f.close()
    cmd += ' %s' % aggregator_file

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
      '1',
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
    appengine_constants.SKIA_TRY_TASK_NAME: process_skia_try_task,
}


class Poller(object):

  def Poll(self):
    info_updated_on = 0
    while True:
      try:
        if (time.time() - info_updated_on) >= UPDATE_INFO_AFTER_SECS:
          log_file = os.path.join(tempfile.gettempdir(), 'update-info.output')
          for cmd in ('bash vm_recover_slaves_from_crashes.sh',):
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
