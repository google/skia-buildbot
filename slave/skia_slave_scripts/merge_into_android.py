#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Merge Skia into Android."""


import os
import sys

from build_step import BuildStep, BuildStepFailure, BuildStepWarning
import skia_vars
from sync_android import ANDROID_CHECKOUT_PATH, REPO, GitAuthenticate
from py.utils.git_utils import GIT
from py.utils import git_utils
from py.utils import misc
from py.utils import shell_utils

SKIA_REPO_URL = skia_vars.GetGlobalVariable('skia_git_url')
SKIA_REV_URL = skia_vars.GetGlobalVariable('revlink_tmpl')

MASTER_SKIA_URL = ('https://googleplex-android-review.googlesource.com/'
                   'platform/external/skia')
MASTER_SKIA_REFS = 'HEAD:refs/heads/master-skia'

UPSTREAM_REMOTE_NAME = 'upstream'

ANDROID_USER_CONFIG = 'include/core/SkUserConfig.h'
UPSTREAM_USER_CONFIG = 'include/config/SkUserConfig.h'

EXTERNAL_SKIA = os.path.join(ANDROID_CHECKOUT_PATH, 'external', 'skia')
# Path to gyp_to_android.py.
PLATFORM_TOOLS_BIN = os.path.join(EXTERNAL_SKIA, 'platform_tools', 'android',
                                  'bin')
sys.path.append(PLATFORM_TOOLS_BIN)
import gyp_to_android

LOCAL_BRANCH_NAME = 'merge'


def RepoAbandon(branch):
  """Run 'repo abandon <branch>'

  'repo abandon' is similar to 'git branch -D', and is only necessary after
  a branch created by 'repo start' is no longer needed."""
  shell_utils.run([REPO, 'abandon', branch])


class MergeIntoAndroid(BuildStep):
  """BuildStep which merges Skia into Android, with a generated Android.mk and
  SkUserConfig.h"""

  def _Run(self):
    with misc.ChDir(EXTERNAL_SKIA):
      # Check to see whether there is an upstream yet.
      if not UPSTREAM_REMOTE_NAME in shell_utils.run([GIT, 'remote', 'show']):
        try:
          shell_utils.run([GIT, 'remote', 'add', UPSTREAM_REMOTE_NAME,
                           SKIA_REPO_URL])
        except shell_utils.CommandFailedException as e:
          if 'remote %s already exists' % UPSTREAM_REMOTE_NAME in e.output:
            # Accept this error. The upstream remote name should have been in
            # the output of git remote show, which would have made us skip this
            # redundant command anyway.
            print ('%s was already added. Why did it not show in git remote'
                   ' show?' % UPSTREAM_REMOTE_NAME)
          else:
            raise e

      # Update the upstream remote.
      shell_utils.run([GIT, 'fetch', UPSTREAM_REMOTE_NAME])

      # Create a stack of commits to submit, one at a time, until we reach a
      # commit that has already been merged.
      commit_stack = []
      head = git_utils.ShortHash('HEAD')

      print 'HEAD is at %s' % head

      if self._got_revision:
        # Merge the revision that started this build.
        commit = git_utils.ShortHash(self._got_revision)
      else:
        raise Exception('This build has no _got_revision to merge!')

      print ('Starting with %s, look for commits that have not been merged to '
             'HEAD' % commit)
      while not git_utils.AIsAncestorOfB(commit, head):
        print 'Adding %s to list of commits to merge.' % commit
        commit_stack.append(commit)
        if git_utils.IsMerge(commit):
          # Skia's commit history is not linear. There is no obvious way to
          # merge each branch in, one commit at a time. So just start with the
          # merge commit.
          print '%s is a merge. Skipping merge of its parents.' % commit
          break
        commit = git_utils.ShortHash(commit + '~1')
      else:
        print '%s has already been merged.' % commit

      if len(commit_stack) == 0:
        raise BuildStepWarning('Nothing to merge; did someone already merge %s?'
                               ' Exiting.' % commit)

      print 'Merging %s commit(s):\n%s' % (len(commit_stack),
                                           '\n'.join(reversed(commit_stack)))

      # Now we have a list of commits to merge.
      while len(commit_stack) > 0:
        commit_to_merge = commit_stack.pop()

        print 'Attempting to merge ' + commit_to_merge

        # Start the merge.
        try:
          shell_utils.run([GIT, 'merge', commit_to_merge, '--no-commit'])
        except shell_utils.CommandFailedException:
          # Merge conflict. There may be a more elegant solution, but for now,
          # undo the merge, and allow (/make) a human to do it.
          git_utils.MergeAbort()
          raise Exception('Failed to merge %s. Fall back to manual human '
                          'merge.' % commit_to_merge)


        # Grab the upstream version of SkUserConfig, which will be used to
        # generate Android's version.
        shell_utils.run([GIT, 'checkout', commit_to_merge, '--',
                         UPSTREAM_USER_CONFIG])

        # We don't want to commit the upstream version, so remove it from the
        # index.
        shell_utils.run([GIT, 'reset', 'HEAD', UPSTREAM_USER_CONFIG])

        # Now generate Android.mk and SkUserConfig.h
        try:
          gyp_to_android.main()
        except AssertionError as e:
          print e
          # Failed to generate the makefiles. Make a human fix the problem.
          git_utils.MergeAbort()
          raise Exception('Failed to generate makefiles for %s. Fall back to '
                          'manual human merge.' % commit_to_merge)

        git_utils.Add('Android.mk')
        git_utils.Add(ANDROID_USER_CONFIG)
        git_utils.Add(os.path.join('tests', 'Android.mk'))
        git_utils.Add(os.path.join('bench', 'Android.mk'))
        git_utils.Add(os.path.join('gm', 'Android.mk'))
        git_utils.Add(os.path.join('dm', 'Android.mk'))

        # Remove upstream user config, which is no longer needed.
        os.remove(UPSTREAM_USER_CONFIG)

        # Create a new branch.
        shell_utils.run([REPO, 'start', LOCAL_BRANCH_NAME, '.'])

        try:
          orig_msg = shell_utils.run([GIT, 'show', commit_to_merge,
                                      '--format="%s"', '-s']).rstrip()
          message = 'Merge %s into master-skia\n\n' + SKIA_REV_URL
          shell_utils.run([GIT, 'commit', '-m', message % (orig_msg,
                                                           commit_to_merge)])
        except shell_utils.CommandFailedException:
          # It is possible that someone else already did the merge (for example,
          # if they are testing a build slave). Clean up and exit.
          RepoAbandon(LOCAL_BRANCH_NAME)
          raise BuildStepWarning('Nothing to merge; did someone already merge '
                '%s?' % commit_to_merge)

        # For some reason, sometimes the bot's authentication from sync_android
        # does not carry over to this step. Authenticate again.
        with GitAuthenticate():
          # Now push to master-skia branch
          try:
            shell_utils.run([GIT, 'push', MASTER_SKIA_URL, MASTER_SKIA_REFS])
          except shell_utils.CommandFailedException:
            # It's possible someone submitted in between our sync and push or
            # push failed for some other reason. Abandon and let the next
            # attempt try again.
            RepoAbandon(LOCAL_BRANCH_NAME)
            raise BuildStepFailure('git push failed!')

          # Our branch is no longer needed. Remove it.
          shell_utils.run([REPO, 'sync', '-j32', '.'])
          shell_utils.run([REPO, 'prune', '.'])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(MergeIntoAndroid))
