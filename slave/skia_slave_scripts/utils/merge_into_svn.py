#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Commit all files within a directory to an SVN repository.

To test:
  cd .../buildbot/slave/skia_slave_scripts/utils
  echo "SvnUsername" >../../../site_config/.svnusername
  echo "SvnPassword" >../../../site_config/.svnpassword
  rm -rf /tmp/svn-source-dir
  mkdir -p /tmp/svn-source-dir
  date >/tmp/svn-source-dir/date.png
  date >/tmp/svn-source-dir/date.txt
  CR_BUILDBOT_PATH=../../../third_party/chromium_buildbot
  PYTHONPATH=$CR_BUILDBOT_PATH/scripts:$CR_BUILDBOT_PATH/site_config \
   python merge_into_svn.py \
   --source_dir_path=/tmp/svn-source-dir \
   --merge_dir_path=/tmp/svn-merge-dir \
   --dest_svn_url=https://skia-autogen.googlecode.com/svn/gm-actual/test \
   --svn_username_file=.svnusername --svn_password_file=.svnpassword
  # and check
  #  http://code.google.com/p/skia-autogen/source/browse/#svn%2Fgm-actual%2Ftest

"""

import misc
import optparse
import os
import shutil
import stat
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

def _DeleteDirectoryContents(dir):
  """Delete all contents (recursively) within dir, but don't delete the
  directory itself.

  @param dir directory whose contents to delete
  """
  basenames = os.listdir(dir)
  for basename in basenames:
    path = os.path.join(dir, basename)
    if os.path.isdir(path):
      shutil.rmtree(path)
    else:
      os.unlink(path)

# TODO: We should either add an Update() command to svn.py, or make its
# _RunSvnCommand() method public; in the meanwhile, abuse its private
# _RunSvnCommand() method.
# See https://code.google.com/p/skia/issues/detail?id=713
def _SvnUpdate(repo, additional_svn_flags=[]):
  return repo._RunSvnCommand(['update'] + additional_svn_flags)

# TODO: We should either add an Import() command to svn.py, or make its
# _RunSvnCommand() method public; in the meanwhile, abuse its private
# _RunSvnCommand() method.
# See https://code.google.com/p/skia/issues/detail?id=713
def _SvnImport(repo, path, url, message, additional_svn_flags=[]):
  return repo._RunSvnCommand(['import', path, url, '--message', message]
                             + additional_svn_flags)

# TODO: We should either add a command like this to svn.py, or make its
# _RunSvnCommand() method public; in the meanwhile, abuse its private
# _RunSvnCommand() method.
# See https://code.google.com/p/skia/issues/detail?id=713
def _SvnDoesUrlExist(repo, url):
  try:
    repo._RunSvnCommand(['ls', url])
    return True
  except Exception:
    # TODO: this will treat *any* exception in "svn ls" as signalling that
    # the URL does not exist.  Should we look for something more specific?
    return False

# TODO: We should either add a command like this to svn.py, or make its
# _RunSvnCommand() method public; in the meanwhile, abuse its private
# _RunSvnCommand() method.
# See https://code.google.com/p/skia/issues/detail?id=713
def _SvnCleanup(repo):
  if not repo._RunSvnCommand(['cleanup']):
    raise Exception('Could not run "svn cleanup"')

def _OnRmtreeError(function, path, excinfo):
  """ onerror function for shutil.rmtree.  If a file is read-only, rmtree will
  fail on Windows.  This function handles the read-only case. """
  if not os.access(path, os.W_OK):
    os.chmod(path, stat.S_IWUSR)
    function(path)
  else:
    raise

def MergeIntoSvn(options):
  """Update an SVN repository with any new/modified files from a directory.

  @param options struct of command-line option values from optparse
  """
  # Get path to SVN username and password files.
  # (Patterned after slave_utils.py's logic to find the .boto file.)
  site_config_path = os.path.join(
      os.path.dirname(__file__), os.pardir, os.pardir, os.pardir, 'site_config')
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

  # If this repo hasn't been created yet, create it, and clear out the mergedir
  # (just in case there is old stuff in there) to match the newly created repo.
  if not _SvnDoesUrlExist(repo=repo, url=options.dest_svn_url):
    _DeleteDirectoryContents(mergedir)
    print _SvnImport(
        repo=repo, url=options.dest_svn_url, path='.',
        message='automatic initial creation by merge_into_svn.py')

  # If we have already checked out this workspace, just update it (resolving
  # any conflicts in favor of the repository HEAD) rather than pulling a
  # fresh checkout.
  if os.path.isdir(os.path.join(mergedir, '.svn')):
    try:
      print _SvnUpdate(repo=repo,
                       additional_svn_flags=['--accept', 'theirs-full'])
    except Exception as e:
      if 'doesn\'t match expected UUID' in ('%s' % e):
        # We occasionally have to reset the repository due to space constraints.
        # In this case, the UUID will change and we have to check out again.
        # Bug: http://code.google.com/p/skia/issues/detail?id=792
        # First, clear the existing directory
        print 'The remote repository UUID has changed.  Removing the existing \
              checkout and checking out again to update with the new UUID'
        shutil.rmtree(mergedir, onerror=_OnRmtreeError)
        os.makedirs(mergedir)
        # Then, check out the repo again.
        print repo.Checkout(url=options.dest_svn_url, path='.')
      elif 'svn cleanup' in ('%s' % e):
        # If a previous commit did not go through, we sometimes end up with a
        # locked working copy and are unable to update.  In this case, run
        # svn cleanup.
        _SvnCleanup(repo)
        print _SvnUpdate(repo=repo,
                         additional_svn_flags=['--accept', 'theirs-full'])
      else:
        raise e
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
  misc.ConfirmOptionsSet({
      '--dest_svn_url': options.dest_svn_url,
      '--source_dir_path': options.source_dir_path,
      '--svn_password_file': options.svn_password_file,
      '--svn_username_file': options.svn_username_file,
      })
  return MergeIntoSvn(options)

if '__main__' == __name__:
  sys.exit(main(None))
