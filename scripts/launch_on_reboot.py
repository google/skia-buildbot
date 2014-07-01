#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Sets up launch-on-reboot behavior."""


import getpass
import os
import socket
import subprocess
import sys
import tempfile


CHECKOUT_ROOT = os.path.realpath(os.path.join(
    os.path.dirname(os.path.abspath(__file__)), os.pardir))

sys.path.append(os.path.join(CHECKOUT_ROOT, 'site_config'))

import slave_hosts_cfg


WINDOWS_STARTUP_PATH = os.path.join(os.path.expanduser('~'), 'AppData',
                                    'Roaming', 'Microsoft', 'Windows',
                                    'Start Menu', 'Programs', 'Startup')


def _build_unix_env_vars(skia_repo_dir=None):
  """Return the set of variables required to run the launch-on-boot script.

  Args:
      skia_repo_dir: optional string; path to the Skia checkout on the
          buildslave. Defaults to the home directory of the current user.

  Returns:
      dictionary containing environment variable names as keys and their values
          as values.
  """
  home = os.path.expanduser('~')
  if not skia_repo_dir:
    skia_repo_dir = home
  return {'SKIA_REPO_DIR': skia_repo_dir,
          'HOME': home}


def _setup_launch_on_reboot_linux(launch_script, skia_repo_dir):
  """Set up launch-on-reboot on Linux.

  Sets up a cron job to run the launch script at reboot.

  Args:
      launch_script: string; the script to launch on boot.
      skia_repo_dir: string; path to the Skia checkout on the buildslave.
  """
  env_vars = _build_unix_env_vars(skia_repo_dir)
  # Wait for the drive containing the launch script to be mounted.
  full_cmd = ('while [ ! -f "%s" ]; do '
                'echo "%s not found"; '
                'sleep 1; '
              'done; ' % (launch_script, launch_script))
  for k, v in env_vars.iteritems():
    full_cmd += 'export %s=%s; ' % (k, v)
  full_cmd += launch_script
  # Write the command to a file to be read by crontab.
  file_contents = '@reboot ' + full_cmd + '\n'
  file_name = None
  try:
    with tempfile.NamedTemporaryFile(delete=False) as reboot_file:
      reboot_file.write(file_contents)
      file_name = reboot_file.name
    if file_name:
      subprocess.check_call(['crontab', '-u', getpass.getuser(), file_name])
  finally:
    if file_name:
      os.remove(file_name)


def _setup_launch_on_reboot_mac(launch_script, skia_repo_dir):
  """Set up launch-on-reboot on Mac.

  Write an XML property list file to be read by launchctl.

  Args:
      launch_script: string; the script to launch on boot.
      skia_repo_dir: string; path to the Skia checkout on the buildslave.
  """
  env_vars = _build_unix_env_vars(skia_repo_dir)
  env_vars_section = '''    <key>EnvironmentVariables</key>
    <dict>'''
  for k, v in env_vars.iteritems():
    env_vars_section += '\n      <key>%s</key><string>%s</string>' % (k, v)
  env_vars_section += '\n    </dict>'
  plist_name = 'com.skiabot.launchonboot'
  plist_contents = '''<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
  <dict>
    <key>Label</key><string>%s</string>
%s
    <key>ProgramArguments</key>
      <array>
      <string>%s</string>
      </array>
    <key>RunAtLoad</key><true/>
    <key>UserName</key><string>chrome-bot</string>
  </dict>
</plist>
''' % (plist_name, env_vars_section, launch_script)
  plist_dir = os.path.join(os.path.expanduser('~'), 'Library', 'LaunchAgents')
  if not os.path.isdir(plist_dir):
    os.makedirs(plist_dir)
  plist_path = os.path.join(plist_dir, plist_name + '.plist')
  with open(plist_path, 'w') as f:
    f.write(plist_contents)


def _setup_launch_on_reboot_win32(launch_script, skia_repo_dir):
  """Set up launch-on-reboot on Windows.

  Creates a batch file in the Startup directory which runs the launch script.

  Args:
      launch_script: string; the script to launch on boot.
      skia_repo_dir: string; path to the Skia checkout on the buildslave.
  """
  bat_contents = ''
  if skia_repo_dir:
    bat_contents += 'set SKIA_REPO_DIR=%s\n' % skia_repo_dir
  bat_contents += launch_script
  bat_path = os.path.join(WINDOWS_STARTUP_PATH, 'launch_on_boot.bat')
  with open(bat_path, 'w') as f:
    f.write(bat_contents)


def setup_launch_on_reboot():
  """Set up launch-on-reboot as appropriate for this platform."""
  # Obtain the slave_host configuration for this buildslave. We're mostly
  # interested in path_to_buildbot, which we'll use to determine skia_repo_dir,
  # and launch_script, which is the script which should be run at boot.
  cfg = slave_hosts_cfg.get_slave_host_config(socket.gethostname())

  # Chop off the last element of path_to_buildbot, since that's the buildbot
  # directory itself.
  if len(cfg.path_to_buildbot) <= 1:
    skia_repo_dir = ''
  else:
    if ':' in cfg.path_to_buildbot[0]:
      # This is an issue with split/joining paths on Windows. If there's a drive
      # letter in the path which gets split, we end up with a list whose first
      # element has 'C:', for example.  os.path.join('C:', 'somedir') does not
      # add a '\'.  Instead, we get 'C:somedir'.  So we add the backslash here.
      cfg.path_to_buildbot[0] += os.path.sep
    skia_repo_dir = os.path.join(*cfg.path_to_buildbot[:-1])

  if cfg.path_to_buildbot[0] == '':
    # If path_to_buildbot begins with '/', path_to_buildbot.split(os.path.sep)
    # will return a list with an empty string as the first element.
    # Unfortunately, os.path.join does not recreate the leading '/', so we have
    # to do it here.
    skia_repo_dir = os.path.sep + skia_repo_dir
  else:
    # Otherwise, this is a relative path
    skia_repo_dir = os.path.join(os.path.expanduser('~'), skia_repo_dir)

  # Use the skia_repo_dir to construct the path to the launch script.
  launch_script = os.path.join(skia_repo_dir, 'buildbot', *cfg.launch_script)

  # Now, call the platform-specific setup function.
  if sys.platform.startswith('linux'):
    _setup_launch_on_reboot_linux(launch_script=launch_script,
                                  skia_repo_dir=skia_repo_dir)
  elif sys.platform == 'darwin':
    _setup_launch_on_reboot_mac(launch_script=launch_script,
                                skia_repo_dir=skia_repo_dir)
  elif sys.platform == 'win32':
    _setup_launch_on_reboot_win32(launch_script=launch_script,
                                  skia_repo_dir=skia_repo_dir)
  else:
    raise NotImplementedError(
        'No defined way to set up launch-on-reboot for %s' % sys.platform)


if __name__ == '__main__':
  setup_launch_on_reboot()
