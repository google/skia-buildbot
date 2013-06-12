#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that polls the skia-telemetry AppEngine WebApp."""


import json
import os
import subprocess
import sys
import tempfile
import time
import urllib

import appengine_constants


SLEEP_BETWEEN_POLLS_SECS = 60

ENCOUNTERED_KEYS = {}


class Poller(object):

  def Poll(self):

    while True:

      # Get all tasks in the queue
      # TODO(rmistry): Make urls to open a tuple??
      get_tasks_page = urllib.urlopen(
          appengine_constants.SKIA_TELEMETRY_WEBAPP +
          appengine_constants.GET_LUA_TASKS_SUBPATH)
      pending_tasks = json.loads(
          get_tasks_page.read().replace('\r\n', '\\r\\n'))
      for key in sorted(pending_tasks.keys()):
        task = pending_tasks[key]
        task_key = task['key']
        if task_key in ENCOUNTERED_KEYS:
          '%s is already being processed' % task_key
          continue
        ENCOUNTERED_KEYS[task_key] = 1
        # Create a run id.
        run_id = '%s-%s' % (task['username'].split('@')[0], time.time())
        lua_file = os.path.join(tempfile.gettempdir(), '%s.lua' % run_id)
        f = open(lua_file, 'w')
        f.write(task['lua_script'])
        f.close()
        # Now call the script!!!
        log_file = os.path.join(tempfile.gettempdir(), '%s.output' % run_id)
        cmd = 'bash vm_run_lua_on_slaves.sh %s %s %s %s' % (
            lua_file, run_id, task['username'], task_key)
        print 'Output will be available in %s' % log_file
        subprocess.Popen(cmd.split(), stdout=open(log_file, 'w'), stderr=open(log_file, 'w'))

      print 'Sleeping %s secs' % SLEEP_BETWEEN_POLLS_SECS
      time.sleep(SLEEP_BETWEEN_POLLS_SECS)


if '__main__' == __name__:
  sys.exit(Poller().Poll())
