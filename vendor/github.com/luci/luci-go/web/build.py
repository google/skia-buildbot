#!/usr/bin/env python
# Copyright 2016 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

"""Manages web/ resource checkout and building.

This script can be run in one of three modes:
  - As "deploy-initialize.py", it will perform resource dependency checkouts
    for "luci_deploy" and quit.
  - As "deploy-build.py", it will build web apps to a "luci_deploy" directory.
  - As "build.py", it is a user-facing tool to manually build web components.
"""

import argparse
import logging
import os
import subprocess
import sys

from distutils.spawn import find_executable

# The root of the "luci-go" checkout, relative to the current "build.py" file.
_LUCI_GO_ROOT = os.path.dirname(os.path.dirname(os.path.abspath(__file__)))

# The default build output path.
_WEB_BUILD_PATH = os.path.join(_LUCI_GO_ROOT, 'web')


def _get_tools():
  tools = {
      'npm': find_executable('npm'),
      'node': find_executable('node'),
  }

  if not (tools['npm'] and tools['node']):
    print """\
Build requires a "node" and "npm" executables to be installed on your local
system. Please install Node.js and NPM. Installation instructions can be found
at:
    https://docs.npmjs.com/getting-started/installing-node
"""
    raise Exception('Unable to locate NPM executable.')

  return tools


def _call_if_outdated(fn, installed, spec, force):
  """Will call "fn" if the file at "install_path" doesn't match "spec".

  If "fn" completes without raising an exception, the "spec" file will be copied
  to the "installed" path, making subsequent runs of this function a
  no-op until the spec file changes..

  Args:
    fn (callable): The function to call if they don't match.
    installed (str): The path to the installed state file.
    spec (str): The path to the source spec file.
    force (bool): If true, call the function regardless.
  """
  with open(spec, 'r') as fd:
    spec_data = fd.read()

  if not force and os.path.isfile(installed):
    with open(installed, 'r') as fd:
      current = fd.read()
    if spec_data == current:
      return

  # Either forcing or out of date.
  fn()

  # Update our installed file to match our spec data.
  with open(installed, 'w') as fd:
    fd.write(spec_data)


def main(argv):
  parser = argparse.ArgumentParser()

  # If we're running the deployment tool version of this script, these arguments
  # are required and positional. Otherwise, they are optional.
  script_name = os.path.basename(sys.argv[0])
  install_only = False
  if script_name == 'deploy-initialize.py':
    # Run as a "luci_deploy" checkout initialization hook.
    parser.add_argument('source_root',
        help='Path to the source root.')
    parser.add_argument('result_path',
        help='(Unused) result path initialization script argument.')
    install_only = True
  elif script_name == 'deploy-build.py':
    # Run as a "luci_deploy" build script.
    parser.add_argument('source_root',
        help='Path to the source root.')
    parser.add_argument('build_dir',
        help='Path to the output build directory. Apps will be written to a '
             '"dist" folder under this path.')
  else:
    # User-facing script.
    parser.add_argument('--source-root', default=_LUCI_GO_ROOT,
        help='Path to the source root.')
    parser.add_argument('--build-dir', default=_WEB_BUILD_PATH,
        help='Path to the output build directory. Apps will be written to a '
             '"dist" folder under this path.')

  parser.add_argument('apps', nargs='*',
      help='Names of the web apps to build. If none are specified, all web '
           'apps will be built.')
  parser.add_argument('-i', '--force-install', action='store_true',
      help='Install NPM/Bower files even if they are considered up-to-date.')
  opts = parser.parse_args(argv)

  # Build our generated web content.
  web_dir = os.path.join(opts.source_root, 'web')
  tools = _get_tools()

  # Install our required Node and Bower modules.
  def install_npm_deps():
    subprocess.check_call([tools['npm'], 'install'], cwd=web_dir,
        stdout=sys.stderr, stderr=sys.stderr)
  _call_if_outdated(
      install_npm_deps,
      os.path.join(web_dir, '.npm.installed'),
      os.path.join(web_dir, 'package.json'),
      opts.force_install)

  def install_bower_deps():
    bower = os.path.join(web_dir, 'node_modules', 'bower', 'bin', 'bower')
    subprocess.check_call([tools['node'], bower, 'install'], cwd=web_dir,
        stdout=sys.stderr, stderr=sys.stderr)
  _call_if_outdated(
      install_bower_deps,
      os.path.join(web_dir, '.bower.installed'),
      os.path.join(web_dir, 'bower.json'),
      opts.force_install)

  # If we're only performing installation, ignore the other arguments.
  if install_only:
    return 0

  # Build requested apps.
  apps = opts.apps
  app_root = os.path.join(web_dir, 'apps')
  if not apps:
    # Get all apps with a gulpfile
    apps = [app for app in os.listdir(app_root)
            if os.path.isfile(os.path.join(app_root, app, 'gulpfile.js'))]

  gulp = os.path.join(web_dir, 'node_modules', 'gulp', 'bin', 'gulp.js')
  for app in apps:
    logging.info('Building app [%s] => [%s]', app, opts.build_dir)
    subprocess.check_call([tools['node'], gulp, '--out', opts.build_dir],
        cwd=os.path.join(app_root, app))

  return 0


if __name__ == '__main__':
  logging.basicConfig(level=logging.DEBUG)
  sys.exit(main(sys.argv[1:]))
