#!/usr/bin/python
# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Initialize the environment variables and start the buildbot slave.
"""

import os
import subprocess
import sys


def remove_all_vars_except(dictionary, keep):
  """Remove all keys from the specified dictionary except those in !keep|"""
  for key in set(dictionary.keys()) - set(keep):
    dictionary.pop(key)


def Reboot():
  print "Rebooting..."
  if sys.platform.startswith('win'):
    subprocess.call(['shutdown', '-r', '-f', '-t', '1'])
  elif sys.platform in ('darwin', 'posix', 'linux2'):
    subprocess.call(['sudo', 'shutdown', '-r', 'now'])
  else:
    raise NotImplementedError('Implement Reboot function')


def HotPatchSlaveBuilder():
  """We could override the SlaveBuilder class but it's way simpler to just
  hotpatch it."""
  from buildbot.slave.bot import SlaveBuilder
  old_remote_shutdown = SlaveBuilder.remote_shutdown

  def rebooting_remote_shutdown(self):
    old_remote_shutdown(self)
    Reboot()

  SlaveBuilder.remote_shutdown = rebooting_remote_shutdown


def main():
  # change the current directory to the directory of the script.
  os.chdir(sys.path[0])

  # Make sure the current python path is absolute.
  paths = os.environ['PYTHONPATH'].split(os.pathsep)
  os.environ['PYTHONPATH'] = ''
  for path in paths:
    os.environ['PYTHONPATH'] += os.path.abspath(path)
    os.environ['PYTHONPATH'] += os.pathsep

  # Update the python path.
  parent_dir = os.path.abspath(os.path.pardir)
  chromium_buildbot_dir = os.path.join(
      parent_dir, 'third_party', 'chromium_buildbot')
  root = os.path.dirname(parent_dir)
  python_path = [
    os.path.join(chromium_buildbot_dir, 'site_config'),
    os.path.join(chromium_buildbot_dir, 'scripts'),
    os.path.join(chromium_buildbot_dir, 'scripts', 'release'),
    os.path.join(chromium_buildbot_dir, 'third_party'),
    os.path.join(root, 'build_internal', 'site_config'),
    os.path.join(root, 'build_internal', 'symsrc'),
    '.',  # Include the current working directory by default.
  ]
  os.environ['PYTHONPATH'] += os.pathsep.join(python_path)

  # Add these in from of the PATH too.
  new_path = python_path
  new_path.extend(sys.path)
  sys.path = new_path

  os.environ['CHROME_HEADLESS'] = '1'
  os.environ['PAGER'] = 'cat'

  # Platform-specific initialization.

  if sys.platform.startswith('win'):
    # list of all variables that we want to keep
    env_var = [
        'APPDATA',
        'BUILDBOT_ARCHIVE_FORCE_SSH',
        'CHROME_HEADLESS',
        'CHROMIUM_BUILD',
        'COMSPEC',
        'COMPUTERNAME',
        'DXSDK_DIR',
        'HOMEDRIVE',
        'HOMEPATH',
        'LOCALAPPDATA',
        'NUMBER_OF_PROCESSORS',
        'OS',
        'PATH',
        'PATHEXT',
        'PROCESSOR_ARCHITECTURE',
        'PROCESSOR_ARCHITEW6432',
        'PROGRAMFILES',
        'PROGRAMW6432',
        'PYTHONPATH',
        'SYSTEMDRIVE',
        'SYSTEMROOT',
        'TEMP',
        'TMP',
        'USERNAME',
        'USERDOMAIN',
        'USERPROFILE',
        'WINDIR',
    ]

    remove_all_vars_except(os.environ, env_var)

    # extend the env variables with the chrome-specific settings.
    depot_tools = os.path.join(chromium_buildbot_dir, '..', 'depot_tools')
    # Reuse the python executable used to start this script.
    python = os.path.dirname(sys.executable)
    system32 = os.path.join(os.environ['SYSTEMROOT'], 'system32')
    wbem = os.path.join(system32, 'WBEM')
    slave_path = [depot_tools, python, system32, wbem]
    # build_internal/tools contains tools we can't redistribute.
    tools = os.path.join(chromium_buildbot_dir, '..', 'build_internal', 'tools')
    if os.path.isdir(tools):
      slave_path.append(os.path.abspath(tools))
    os.environ['PATH'] = os.pathsep.join(slave_path)
    os.environ['LOGNAME'] = os.environ['USERNAME']

  elif sys.platform in ('darwin', 'posix', 'linux2'):
    # list of all variables that we want to keep
    env_var = [
        'CCACHE_DIR',
        'CHROME_ALLOCATOR',
        'CHROME_HEADLESS',
        'DISPLAY',
        'DISTCC_DIR',
        'HOME',
        'HOSTNAME',
        'LANG',
        'LOGNAME',
        'PAGER',
        'PATH',
        'PWD',
        'PYTHONPATH',
        'SHELL',
        'SSH_AGENT_PID',
        'SSH_AUTH_SOCK',
        'SSH_CLIENT',
        'SSH_CONNECTION',
        'SSH_TTY',
        'USER',
        'USERNAME',
    ]

    remove_all_vars_except(os.environ, env_var)

    depot_tools = os.path.join(chromium_buildbot_dir, '..', 'depot_tools')

    slave_path = [depot_tools, '/usr/bin', '/bin',
                  '/usr/sbin', '/sbin', '/usr/local/bin']
    os.environ['PATH'] = os.pathsep.join(slave_path)

  elif sys.platform == 'cygwin':
    #TODO(maruel): Implement me.
    pass
  else:
    raise NotImplementedError('Unknown platform')

  # Run the slave.
  HotPatchSlaveBuilder()
  import twisted.scripts.twistd as twistd
  twistd.run()


if '__main__' == __name__:
  main()
