#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Merge Skia into Android."""


import os
import sys

from build_step import BuildStep
from sync_android import ANDROID_CHECKOUT_PATH, REPO
from utils import shell_utils


SKIA_REPO_URL = 'https://skia.googlesource.com/skia'
MASTER_SKIA_URL = ('https://googleplex-android-review.googlesource.com/'
                   'platform/external/skia')
MASTER_SKIA_REFS = 'HEAD:refs/heads/master-skia'

UPSTREAM_REMOTE_NAME = 'upstream'
UPSTREAM_BRANCH_NAME = UPSTREAM_REMOTE_NAME + '/master'

ANDROID_USER_CONFIG = 'include/core/SkUserConfig.h'
UPSTREAM_USER_CONFIG = 'include/config/SkUserConfig.h'

EXTERNAL_SKIA = os.path.join(ANDROID_CHECKOUT_PATH, 'external', 'skia')
# Path to gyp_to_android.py, relative to EXTERNAL_SKIA.
PLATFORM_TOOLS_BIN = os.path.join('platform_tools', 'android', 'bin')

# Used to determine the revision number, which will be entered as part of the
# commit message. TODO (scroggo): What should the commit message say when we
# switch to git and there is no revision number? (Or will there be one? See
# skbug.com/1639).
GIT_SVN_ID = 'http://skia.googlecode.com/svn/trunk@'
GIT = 'git'

class MergeIntoAndroid(BuildStep):
  """BuildStep which merges Skia into Android, with a generated Android.mk and
  SkUserConfig.h"""

  def _Run(self):
    # TODO: The code below this depends on the CWD being set to EXTERNAL_SKIA,
    # but calling os.chdir() seems problematic. Will our calling code be upset
    # by the fact that we changed the directory? What if a function we call
    # changes the CWD? If shell_utils.run() took a cwd parameter, we could
    # avoid the chdir.
    print 'cd %s' % EXTERNAL_SKIA
    os.chdir(EXTERNAL_SKIA)

    # Set up git config properly.
    shell_utils.run([GIT, 'config', '--global', 'user.email',
                     '"31977622648@project.gserviceaccount.com"'])
    shell_utils.run([GIT, 'config', '--global', 'user.name',
                     '"Skia_Android Canary Bot"'])

    # Check to see whether there is an upstream yet.
    if not UPSTREAM_REMOTE_NAME in shell_utils.run([GIT, 'remote', 'show']):
      shell_utils.run([GIT, 'remote', 'add', UPSTREAM_REMOTE_NAME,
                       SKIA_REPO_URL])

    # Update the upstream remote.
    shell_utils.run([GIT, 'fetch', UPSTREAM_REMOTE_NAME])

    # Start the merge.
    shell_utils.run([GIT, 'merge', UPSTREAM_BRANCH_NAME, '--no-commit'])

    # FIXME (scroggo): If we put all of Skia into Android (see skbug.com/2416),
    # it might make sense to do a commit now, and use a separate build step for
    # generating Android.mk and SkUserConfig.h.

    # Grab the upstream version of SkUserConfig, which will be used to
    # generate Android's version.
    shell_utils.run([GIT, 'checkout', UPSTREAM_BRANCH_NAME, '--',
                     UPSTREAM_USER_CONFIG])

    # We don't want to commit the upstream version, so remove it from the index.
    shell_utils.run([GIT, 'reset', 'HEAD', UPSTREAM_USER_CONFIG])

    # Now generate Android.mk and SkUserConfig.h
    sys.path.append(os.path.join(os.getcwd(), PLATFORM_TOOLS_BIN))
    import gyp_to_android
    gyp_to_android.main()
    shell_utils.run([GIT, 'add', 'Android.mk'])
    shell_utils.run([GIT, 'add', ANDROID_USER_CONFIG])

    # Remove the files Android doesn't want or need.
    # FIXME: This complicates the merge process. Is it worth it?
    # See skbug.com/2416.
    shell_utils.run([GIT, 'rm', '-rf', 'DEPS', 'Doxyfile', 'Makefile',
                     'make.py', 'debugger/', 'experimental/', 'samplecode/',
                     'platform_tools/'])

    # Now check to see that we haven't left any merge conflicts.
    if len(shell_utils.run([GIT, 'ls-files', '-u'])) > 0:
      # FIXME (scroggo): We're now in a bad state. How does someone fix this?
      # Log into the bot and clean it up? How do we stop the bot from
      # continuing to attempt merges?
      # For now, abandon the merge, so the next attempt can start cleanly.
      shell_utils.run([GIT, 'merge', '--abort'])
      raise Exception('Merge needs fixing!')

    # Create a new branch.
    shell_utils.run([REPO, 'start', 'merge', '.'])

    # Figure out the revision being merged, and use it to write the commit
    # message.
    commit_message = shell_utils.run([GIT, 'show', UPSTREAM_BRANCH_NAME, '-q'])
    rev_index = commit_message.find(GIT_SVN_ID) + len(GIT_SVN_ID)
    rev_end = commit_message.find(' ', rev_index)
    revision = commit_message[rev_index:rev_end]
    shell_utils.run([GIT, 'commit', '-m', 'Merge Skia at r' + revision])

    # Now push to master-skia branch
    shell_utils.run([GIT, 'push', MASTER_SKIA_URL, MASTER_SKIA_REFS])

    # Remove upstream user config, which is no longer needed.
    shell_utils.run(['rm', UPSTREAM_USER_CONFIG])

    # Our branch is no longer needed. Remove it.
    shell_utils.run([REPO, 'sync', '-j32', '.'])
    shell_utils.run([REPO, 'prune', '.'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(MergeIntoAndroid))
