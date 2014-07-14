#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Restart the build masters."""


import os
import posixpath
import sys
import threading
import time
import urllib2

BUILDBOT_PATH = os.path.realpath(os.path.join(os.path.dirname(__file__),
                                              os.pardir))
sys.path.append(BUILDBOT_PATH)
sys.path.append(os.path.join(BUILDBOT_PATH, 'site_config'))
sys.path.append(os.path.join(BUILDBOT_PATH, 'third_party', 'chromium_buildbot',
                             'site_config'))

from common.py.utils import shell_utils
import config_private
import slave_hosts_cfg


# File where the PID of the running master is stored.
# TODO(borenet): Store the master information in slave_hosts_cfg.py (rename it).
PID_FILE = posixpath.join('skia-repo', 'buildbot', 'master', 'twistd.pid')

# Number of seconds to wait between checks of whether the master has restarted.
RESTART_POLL_INTERVAL = 5

# Maximum number of seconds allowed for the master to restart.
RESTART_TIMEOUT = 180


class NoRedirectHandler(urllib2.HTTPErrorProcessor):
  """Handler which does not follow redirects."""
  def http_response(self, req, resp):
    return resp


def get_running_pid(master):
  master_hostname = master.master_fqdn.split('.')[0]
  pid_cmd = slave_hosts_cfg.compute_engine_login(master_hostname, None)
  pid_cmd.extend(['cat', PID_FILE])
  try:
    return shell_utils.run(pid_cmd, echo=False).splitlines()[-1]
  except shell_utils.CommandFailedException as e:
    if 'No such file or directory' in e.output:
      return None
    raise


def restart_master(master):
  """Restart the given master.

  Log in to the master host, read the PID file, submit a "clean restart"
  request, and wait until the master restarts.

  Args:
      master: config_private.Master.*; the master to restart.
  """
  # Obtain the master process PID.
  old_pid = get_running_pid(master)

  # Submit the "clean restart" request.
  shutdown_url = 'http://%s:%s/shutdown' % (master.master_host,
                                            master.master_port)
  print '%s: Sending shutdown request to %s' % (master.project_name,
                                                shutdown_url)
  # Don't follow redirects, since the master might shut down before we can get
  # a response from the page to which we're redirected.
  urllib2.build_opener(NoRedirectHandler).open(shutdown_url)

  # Wait until the master restarts.
  start = time.time()
  while True:
    new_pid = get_running_pid(master)
    if new_pid:
      if new_pid != old_pid:
        print '%s finished restarting.' % master.project_name
        return
      print '%s is still running.' % master.project_name
    else:
      print '%s has shut down but has not yet restarted.' % master.project_name
    if time.time() - start > RESTART_TIMEOUT:
      if new_pid:
        msg = ('%s failed to shut down within %d seconds.' % (
                   master.project_name, RESTART_TIMEOUT))
      else:
        msg = ('%s shut down but failed to restart within %d seconds' % (
                   master.project_name, RESTART_TIMEOUT))
      raise Exception(msg)
    time.sleep(RESTART_POLL_INTERVAL)


def main():
  """Restart the build masters."""
  threads = []

  for master in config_private.Master.valid_masters:
    thread = threading.Thread(target=restart_master, args=(master,))
    thread.daemon = True
    threads.append(thread)
    thread.start()

  for thread in threads:
    thread.join()


if '__main__' == __name__:
  main()
