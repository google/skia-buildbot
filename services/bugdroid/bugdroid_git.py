#!/usr/bin/python
# Copyright (c) 2010 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Bugdroid Git implementation for the Skia repositories.

Execute the script with the following command:
python bugdroid_git.py --repo /storage/skia-repos/skia \
                       --log-file-name skia-bugdroid-log.txt
"""

import logging
import optparse
import os
import sys
import time

from bugdroid import Bugdroid

# Set the PYTHONPATH for this script to include skia_slave_scripts.utils.
sys.path.append(
    os.path.join(os.path.dirname(os.path.realpath(__file__)), os.pardir,
                 os.pardir, 'slave', 'skia_slave_scripts', 'utils'))
import shell_utils


# Time to sleep between repository polls.
SLEEP_BETWEEN_POLLS = 60
# The user BugDroid is run as
BUGDROID_USER = 'skia-commit-bot@chromium.org'


def main():
  parser = optparse.OptionParser()
  parser.add_option('-a', '--allowed-projects', action='append',
                    default=['skia'],
                    dest='trackers',
                    help='issue trackers that can be updated')
  parser.add_option('-e', '--default-project', dest='default_tracker',
                    help='the issue tracker to update if no project name is '
                         'given in the BUG= line', default='skia')
  parser.add_option('-c', '--rev-link', dest='rev_link',
                    help='the link to the committed revision in the repository '
                         'Eg: https://skia.googlesource.com/skia/+/')
  parser.add_option('-v', '--version', dest='version_string',
                    help='the version string to be printed in the log.',
                    default='1.0')
  parser.add_option('-r', '--repo', dest='repo_location',
                    help='the complete path to the git repository bugdroid '
                         'should track.')
  parser.add_option('-l', '--log-file-name', dest='log_file_name',
                    help='the name of the bugdroid log file, it will be '
                         'created in the same directory as this file.')
  (options, _args) = parser.parse_args()

  # Validate arguments.
  repo_location = options.repo_location
  if not repo_location or not os.path.exists(repo_location):
    raise Exception('Must specify a valid path to a repository using --repo')

  # Configure the logger
  folder_path = os.path.dirname(os.path.abspath(__file__))
  log_file_path = os.path.join(folder_path, options.log_file_name)
  logging.basicConfig(filename=log_file_path, level=logging.DEBUG, maxBytes=10)

  logging.debug('===========================================')
  logging.debug(time.strftime('Bugdroid starting: %Y-%m-%d %H:%M:%S'))
  logging.debug('Current bugdroid version %s' % options.version_string)

  # Find and use the .bugdroid_password file from the toplevel buildbot dir.
  parent = os.path.dirname(os.path.abspath(__file__))
  password_path = os.path.join(parent, os.pardir, os.pardir,
                               '.bugdroid_password')
  if os.path.exists(password_path):
    logging.debug('Using the local password file: %s' % password_path)
    password = open(password_path, 'r').readline().strip()
  else:
    logging.debug('No password file present, aborting!')
    return 1
  bugger = Bugdroid(BUGDROID_USER, password)
  if not bugger.login():
    logging.debug('Login failed, aborting.')
    return 1

  # Keep going till the process is killed by a Keyboard Interrupt.
  try:
    while True:
      # Change cwd to the repo_location.
      os.chdir(repo_location)

      # Do a git fetch on the specified repository.
      git_fetch_cmd = ['git', 'fetch']
      shell_utils.run(git_fetch_cmd)

      # Find all new commits.
      git_rev_list_cmd = [
          'git',
          'rev-list',
          '--topo-order',
          'HEAD..origin/master',
      ]
      remaining_hashes = shell_utils.run(git_rev_list_cmd)
      if remaining_hashes:
        remaining_hashes = remaining_hashes.split()
      while remaining_hashes:
        next_hash = remaining_hashes.pop()

        # Checkout the next commit hash.
        git_checkout_cmd = [
            'git',
            'checkout',
            next_hash
        ]
        shell_utils.run(git_checkout_cmd)

        # Run Bugdroid using the content_info of the hash.
        get_content_info = [
            'git',
            'show',
            '--pretty=format:Commit: ' +
                options.rev_link + '%h%n Email: %ae%n%n%s%n%n%b%n%ad',
            '--name-status'
        ]
        content = shell_utils.run(get_content_info)
        if not bugger.process_all_bugs(content, options.trackers,
                                       options.default_tracker):
          logging.debug('No bug ID was found in %s.', next_hash)

      # Wait for a minute before trying everything again
      logging.debug('Sleeping %s seconds before trying again.',
                    SLEEP_BETWEEN_POLLS)
      time.sleep(SLEEP_BETWEEN_POLLS)

  except KeyboardInterrupt as e:
    logging.exception(e)

  logging.debug(time.strftime('Bugdroid exiting: %Y-%m-%d %H:%M:%S'))


if __name__ == '__main__':
  main()

