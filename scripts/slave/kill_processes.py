#!/usr/bin/python
# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
#
# TODO(epoger): temporarily copied over from
# third_party/chromium_buildbot/scripts/slave
# to work around failure in "taskkill" step on Windows buildbot.
# The root problem is thatfactory/commands.FactoryCommands.__init__()
# assumes that kill_processes.py can be found in the scripts/slave directory
# (as opposed to third_party/chromium_buildbot/scripts/slave).
#
# A good long-term fix might be to add an optional script_dir parameter to
# FactoryCommands.__init__() so that callers can change where it looks for these
# scripts.

"""A tool to kill any leftover test processes, executed by buildbot.

Only works on Windows."""

import os
import subprocess
import sys
import time


def KillAll(process_names, must_die=True):
  """Tries to kill all copies of each process in the processes list."""
  killed_processes = []

  for process_name in process_names:
    if ProcessExists(process_name):
      Kill(process_name)
      killed_processes.append(process_name)

  # If we allow any processes to continue after trying to kill them, return
  # now.
  if not must_die:
    return True

  # We require that all processes we tried to kill must be killed.  Let's
  # verify that.

  retcode = True
  if killed_processes:
    # Sleep for 5 seconds to give any killed processes time to shut down.
    time.sleep(5)

    # Check if the process is still running.  If it is, it can't easily be
    # killed and something is broken.  Return this error in our exit code.
    for process_name in killed_processes:
      retcode = (retcode and not ProcessExists(process_name))
  return retcode


def ProcessExists(process_name):
  """Return whether process_name is found in tasklist output."""
  # Use tasklist.exe to find if a given process_name is running.
  command = ('tasklist.exe /fi "imagename eq %s" | findstr.exe "K"' %
             process_name)
  # findstr.exe exits with code 0 if the given string is found.
  return os.system(command) == 0


def Kill(process_name):
  command = ['taskkill.exe', '/f', '/t', '/im']
  subprocess.call(command + [process_name])


# rdpclip.exe is part of Remote Desktop.  It has a bug that sometimes causes
# it to keep the clipboard open forever, denying other processes access to it.
# Killing BuildConsole.exe usually stops an IB build within a few seconds.
# Unfortunately, killing devenv.com or devenv.exe doesn't stop a VS build, so
# we don't bother pretending.
processes = [
    # Utilities we don't build, but which we use or otherwise can't
    # have hanging around.
    'BuildConsole.exe',
    'httpd.exe',
    'outlook.exe',
    'perl.exe',
    'python_slave.exe',
    'rdpclip.exe',
    'svn.exe',

    # These processes are spawned by some tests and should be killed by same.
    # It may occur that they are left dangling if a test crashes, so we kill
    # them here too.
    'firefox.exe',
    'iexplore.exe',
    #'ieuser.exe',
    'acrord32.exe',

    # The JIT debugger may start when devenv.exe crashes.
    'vsjitdebugger.exe',
    # This process is also crashing once in a while during compile.
    'midlc.exe',

    # Things built by/for Chromium.
    'base_unittests.exe',
    'cacheinvalidation_unittests.exe',
    'ceee_broker.exe',
    'ceee_common_unittests.exe',
    'chrome.exe',
    'chrome_frame_helper.exe',
    'chrome_frame_net_tests.exe',
    'chrome_frame_tests.exe',
    'chrome_launcher.exe',
    'chrome_frame_unittests.exe',
    'crash_service.exe',
    'crypto_unittests.exe',
    'debug_message.exe',
    'DumpRenderTree.exe',
    'flush_cache.exe',
    'gpu_tests.exe',
    'ie_unittests.exe',
    'image_diff.exe',
    'installer_util_unittests.exe',
    'interactive_ui_tests.exe',
    'ipc_tests.exe',
    'jingle_unittests.exe',
    'mediumtest_ie.exe',
    'memory_test.exe',
    'net_unittests.exe',
    'page_cycler_tests.exe',
    'perf_tests.exe',
    'plugin_tests.exe',
    'printing_unittests.exe',
    'reliability_tests.exe',
    'selenium_tests.exe',
    'startup_tests.exe',
    'sync_integration_tests.exe',
    'tab_switching_test.exe',
    'test_shell.exe',
    'test_shell_tests.exe',
    'tld_cleanup.exe',
    'ui_tests.exe',
    'unit_tests.exe',
    'v8_shell.exe',
    'v8_mksnapshot.exe',
    'v8_shell_sample.exe',
    'wow_helper.exe',
]

# Some processes may be present occasionally unrelated to the current build.
# For these, it's not an error if we attempt to kill them and they don't go
# away.
lingering_processes = [
    # When VC crashes during compilation, this process which manages the .pdb
    # file generation sometime hangs.  However, Incredibuild will spawn
    # mspdbsrv.exe, so don't trigger an error if it is still present after
    # we attempt to kill it.
    'mspdbsrv.exe',
]

if '__main__' == __name__:
  # Kill all lingering processes.  It's okay if these aren't killed or end up
  # reappearing.
  KillAll(lingering_processes, must_die=False)

  # Kill all regular processes.  We must guarantee that these are killed since
  # we exit with an error code if they're not.
  if KillAll(processes, must_die=True):
    sys.exit(0)

  # Some processes were not killed, exit with non-zero status.
  sys.exit(1)
