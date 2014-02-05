#!/usr/bin/python
# Copyright (c) 2010 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import logging
import optparse
import os
import sys
import time
from bugdroid import Bugdroid

def _get_change_info(commit_hash):
  """ Create content information for Issue Tracking System. """
  get_content_info = ['git',
                      'show',
                      '--pretty="format:Commit: %H%n Email: %ae%n%n%s%n%n%b"',
                      '--name-status',
                      commit_hash]
  content = os.popen(' '.join(get_content_info)).read()
  return content

def main():
  parser = optparse.OptionParser()
  parser.add_option('-c', '--change-string-path', dest='file_path',
                    help='file path to a text file that contains the change '
                          'info.  this is for debugging only.', metavar='FILE')
  parser.add_option('-a', '--allowed-projects', action='append',
                    default=['chromium', 'chromium-os', 'gyp', 'nativeclient',
                             'chrome-os-partner'],
                    dest='trackers',
                    help='issue trackers that can be updated')
  parser.add_option('-e', '--default-project', dest='default_tracker',
                    help='the issue tracker to update if no project name is '
                         'given in the BUG= line', default='chromium-os')
  parser.add_option('-p', '--password-file', dest='password_file_path',
                    help='file path to a text file that contains the password'
                         'for the account.', metavar='FILE')
  parser.add_option('-v', '--version', dest='version_string',
                    help='the version string to be printed in the log.',
                    default='1.5')
  (options, args) = parser.parse_args()

  # Configure the logger
  folder_path = os.path.dirname(os.path.abspath(__file__))
  log_file_path = os.path.join(folder_path, 'bugdroid_log.txt')
  logging.basicConfig(filename=log_file_path, level=logging.DEBUG, maxBytes=10)

  logging.debug('===========================================')
  logging.debug(time.strftime('Bugdroid starting: %Y-%m-%d %H:%M:%S'))
  logging.debug('Current bugdroid version %s' % options.version_string)
  if not options.password_file_path:
    parent = os.path.dirname(os.path.abspath(__file__))
    password_path = os.path.join(parent, '.bugdroid_password')
    password = None
    if os.path.exists(password_path):
      logging.debug('Using the local password file: %s' % password_path)
      password = open(password_path, 'r').readline().strip()
    else:
      logging.debug('No password file present, aborting!')
      return 1
  else:
    if os.path.exists(options.password_file_path):
      password = open(options.password_file_path, 'r').readline().strip()
    else:
      logging.debug('Password file path is invalid! Path: %s\nExiting.' %
                    options.password_file_path)
      parser.error('Password file path is invalid')

  bugger = Bugdroid('bugdroid1@chromium.org', password,
                    svn_trackers_to_ignore=['chromium', 'chrome',
                                            'nativeclient', 'webrtc'])
  if not bugger.login():
    logging.debug('Login failed, aborting.')
    return 1

  content = None
  if not options.file_path:
    for line in sys.stdin:
      logging.debug('Raw input: %s' % line)
      line = line.strip()
      logging.debug('Stripped input: %s' % line)
      # Obtain the repo we are in so we can print it out for debugging
      logging.debug('Repo directory: %s' % os.getcwd())
      if not line:
        continue
      data = line.split()
      # Git can send a large list of hashes and paths that can look something
      # like <hash> <hash> refs/heads/master <hash> <hash> /ref/heads/10.3B ...
      commit_hashes = data[1:len(data):3]
      for commit_hash in commit_hashes:
        logging.debug('New hash: %s' % commit_hash)
        content = _get_change_info(commit_hash)
        if content and not bugger.process_all_bugs(content, options.trackers,
                                                   options.default_tracker):
          logging.debug('No bug ID was found.')
  else:
    if not os.path.isfile(options.file_path):
      print 'The path given is invalid'
      return 1
    f = open(options.file_path, 'rU')
    content = str(f.read())
    if content == None:
      logging.debug('There was no content to parse, aborting.')
      return 1
    if not bugger.process_all_bugs(content, options.trackers,
                                   options.default_tracker):
      logging.debug('No bug ID was found.')
  logging.debug(time.strftime('Bugdroid exiting: %Y-%m-%d %H:%M:%S'))
  return 0

if __name__ == '__main__':
  main()

