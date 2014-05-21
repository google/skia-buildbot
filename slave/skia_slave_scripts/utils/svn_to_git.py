#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Translates SVN revision numbers to Git commit hashes."""


from bisect import bisect_left
import re
import subprocess
import sys


ORIGIN_MASTER = 'origin/master'


class GitCommitNotFoundError(Exception):
  """Raised by GitHashFromSvnRev when no associated Git commit is found."""
  pass


class MissingGitSvnIDError(Exception):
  """Raised by GitHashFromSvnRev when a Git commit has no git-svn-id."""
  pass


def GitHashFromSvnRev(desired_svn_rev):
  """Try to obtain the Git commit associated with the given SVN revision.

  Assumes that CWD is inside the Git repository in question.

  Args:
      desired_svn_rev: string; SVN revision number.
  Returns:
      string; the hash pointing to the Git commit associated with the given SVN
          revision.
  Raises:
      GitCommitNotFoundError if no Git commit associated with the given SVN
          revision is found.
      MissingGitSvnIDError if a Git commit has no git-svn-id.
      subprocess.CalledProcessError if any Git command fails unexpectedly.
  """
  class GitCommit(object):
    def __init__(self, commit_hash, svn_rev=None):
      """Construct the GitCommit.

      Args:
          commit_hash: string; commit hash for this commit.
          svn_rev: string; SVN revision number associated with this commit. If
              not provided, will be lazily evaluated because very few of them
              are actually needed (lg N for binary search).
      """
      self._commit_hash = commit_hash
      self._svn_rev = svn_rev

    @property
    def commit_hash(self):
      return self._commit_hash

    @property
    def svn_rev(self):
      """Find the SVN revision associated with the Git commit."""
      if not self._svn_rev:
        git_log = subprocess.check_output(
            ['git', 'show', '-s', self._commit_hash])
        match = re.search('^\s*git-svn-id:.*@(?P<svn_revision>\d+)\ ', git_log,
                          re.MULTILINE)
        if not match:
          # This should never happen in a repo which is entirely auto-merged
          # from an SVN repo.
          raise MissingGitSvnIDError('%s has no git-svn-id!' % commit)
        self._svn_rev = match.group('svn_revision')
      return self._svn_rev

    def __lt__(self, other):
      return int(self.svn_rev) < int(other.svn_rev)

  # Try to limit the number of commits we have to search.
  commits_to_load = \
      int(GitCommit('origin/master').svn_rev) - int(desired_svn_rev) + 1
  if commits_to_load < 1:
    raise GitCommitNotFoundError('%s not found in any Git commit!' %
                                 desired_svn_rev)
  commit_hashes = subprocess.check_output(
      ['git', 'rev-list', '--max-count=%d' % commits_to_load, ORIGIN_MASTER]
      ).splitlines()
  commit_hashes.reverse()
  all_commits = [GitCommit(commit) for commit in commit_hashes]

  commit_index = bisect_left(all_commits,
                             GitCommit(None, svn_rev=desired_svn_rev))
  try:
    if all_commits[commit_index].svn_rev == desired_svn_rev:
      return all_commits[commit_index].commit_hash
  except IndexError:
    # If the found index is out of bounds, we couldn't find the commit.
    pass
  raise GitCommitNotFoundError('%s not found in any Git commit!' %
                               desired_svn_rev)


if __name__ == '__main__':
  try:
    sys.stdout.write(GitHashFromSvnRev(sys.argv[1]))
  except Exception as e:
    print e
    sys.exit(1)
