#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Commit all files within a directory to an SVN repository.

To test:
  cd .../buildbot/slave/skia_slave_scripts
  echo "SvnUsername" >../../site_config/.svnusername
  echo "SvnPassword" >../../site_config/.svnpassword
  rm -rf /tmp/svnmerge
  mkdir -p /tmp/svnmerge
  date >/tmp/svnmerge/date.png
  date >/tmp/svnmerge/date.txt
  CR_BUILDBOT_PATH=../../third_party/chromium_buildbot
  PYTHONPATH=$CR_BUILDBOT_PATH/scripts:$CR_BUILDBOT_PATH/site_config \
   python merge_into_svn.py \
   --source_dir_path=/tmp/svnmerge \
   --dest_svn_url=https://skia-autogen.googlecode.com/svn/gm-actual/test \
   --svn_username_file=.svnusername --svn_password_file=.svnpassword
  # and check
  #  http://code.google.com/p/skia-autogen/source/browse/#svn%2Fgm-actual%2Ftest

"""

import optparse
import os
import shutil
import skia_slave_utils
import sys
import tempfile

from slave import svn


def ReadFirstLineOfFileAsString(filename):
  """Safely return the first line of a file as a string.
  If there is an exception, it will be raised to the caller, but we will
  close the file handle first.

  @param filename path to the file to read
  """
  f = open(filename, 'r')
  try:
    contents = f.readline().splitlines()[0]
  finally:
    f.close()
  return contents

def _CopyAllFiles(source_dir, dest_dir):
  """Copy all files from source_dir into dest_dir.

  Recursively copies files from directories as well.
  Skips .svn directories.

  @param source_dir
  @param dest_dir
  """
  basenames = os.listdir(source_dir)
  for basename in basenames:
    source_path = os.path.join(source_dir, basename)
    dest_path = os.path.join(dest_dir, basename)
    if basename == '.svn':
      continue
    if os.path.isdir(source_path):
      _CopyAllFiles(source_path, dest_path)
    else:
      shutil.copyfile(source_path, dest_path)

def MergeIntoSvn(options):
  """Update an SVN repository with any new/modified files from a directory.

  @param options struct of command-line option values from optparse
  """
  # Get path to SVN username and password files.
  # (Patterned after slave_utils.py's logic to find the .boto file.)
  site_config_path = os.path.join(os.path.dirname(__file__),
                                  '..', '..', 'site_config')
  svn_username_path = os.path.join(site_config_path, options.svn_username_file)
  svn_password_path = os.path.join(site_config_path, options.svn_password_file)
  svn_username = ReadFirstLineOfFileAsString(svn_username_path).rstrip()
  svn_password = ReadFirstLineOfFileAsString(svn_password_path).rstrip()

  # Check out the SVN repository into the merge dir.
  # (If no merge dir was specified, use a temporary dir.)
  if options.merge_dir_path:
    mergedir = options.merge_dir_path
  else:
    mergedir = tempfile.mkdtemp()
  if not os.path.isdir(mergedir):
    os.makedirs(mergedir)
  repo = svn.Svn(directory=mergedir,
                 username=svn_username, password=svn_password,
                 additional_svn_flags=[
                     '--trust-server-cert', '--no-auth-cache',
                     '--non-interactive'])

  # If we have already checked out this workspace, just update it (resolving
  # any conflicts in favor of the repository HEAD) rather than pulling a
  # fresh checkout.
  if os.path.isdir(os.path.join(mergedir, '.svn')):
    # TODO: We should either add an Update() command to svn.py, or make its
    # _RunSvnCommand() method public; in the meanwhile, abuse its private
    # _RunSvnCommand() method.
    print repo._RunSvnCommand(['update', '--accept', 'theirs-full'])
  else:
    print repo.Checkout(url=options.dest_svn_url, path='.')

  # Copy in all the files we want to update/add to the repository.
  _CopyAllFiles(source_dir=options.source_dir_path, dest_dir=mergedir)

  # Make sure all files are added to SVN and have the correct properties set.
  repo.AddFiles(repo.GetNewFiles())
  repo.SetPropertyByFilenamePattern('*.png', 'svn:mime-type', 'image/png')
  repo.SetPropertyByFilenamePattern('*.pdf', 'svn:mime-type', 'application/pdf')

  # Commit changes to the SVN repository and clean up.
  print repo.Commit(message=options.commit_message)
  if not options.merge_dir_path:
    print 'deleting mergedir %s' % mergedir
    shutil.rmtree(mergedir, ignore_errors=True)
  return 0

def main(argv):
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '--commit_message', default='merge_into_svn automated commit',
      help='message to log within SVN commit operation')
  option_parser.add_option(
      '--dest_svn_url',
      help='URL pointing to SVN directory where we want to commit the files')
  option_parser.add_option(
      '--merge_dir_path',
      help='path within which to make a local checkout and merge contents;'
           ' if this directory already contains a checkout of the SVN repo,'
           ' it will be updated (rather than a complete fresh checkout pulled)'
           ' to speed up the merge process.'
           ' If this option is not specified, a temp directory will be created'
           ' and a complete fresh checkout will be pulled')
  option_parser.add_option(
      '--source_dir_path',
      help='full path of the directory whose contents we wish to commit')
  option_parser.add_option(
      '--svn_password_file',
      help='file (within site_config dir) from which to read the SVN password')
  option_parser.add_option(
      '--svn_username_file',
      help='file (within site_config dir) from which to read the SVN username')
  (options, args) = option_parser.parse_args()
  if len(args) != 0:
    raise Exception('bogus command-line argument; rerun with --help')
  skia_slave_utils.ConfirmOptionsSet({
      '--dest_svn_url': options.dest_svn_url,
      '--source_dir_path': options.source_dir_path,
      '--svn_password_file': options.svn_password_file,
      '--svn_username_file': options.svn_username_file,
      })
  return MergeIntoSvn(options)

if '__main__' == __name__:
  sys.exit(main(None))
