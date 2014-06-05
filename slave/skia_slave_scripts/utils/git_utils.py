#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains functions for using git."""


import os
import shell_utils


GIT = 'git.bat' if os.name == 'nt' else 'git'


def Add(addition):
  """Run 'git add <addition>'"""
  shell_utils.run([GIT, 'add', addition])

def AIsAncestorOfB(a, b):
  """Return true if a is an ancestor of b."""
  return shell_utils.run([GIT, 'merge-base', a, b]).rstrip() == FullHash(a)

def FullHash(commit):
  """Return full hash of specified commit."""
  return shell_utils.run([GIT, 'rev-parse', '--verify', commit]).rstrip()

def IsMerge(commit):
  """Return True if the commit is a merge, False otherwise."""
  rev_parse = shell_utils.run([GIT, 'rev-parse', commit, '--max-count=1',
                               '--no-merges'])
  last_non_merge = rev_parse.split('\n')[0]
  # Get full hash since that is what was returned by rev-parse.
  return FullHash(commit) != last_non_merge

def MergeAbort():
  """Abort in process merge."""
  shell_utils.run([GIT, 'merge', '--abort'])

def ShortHash(commit):
  """Return short hash of the specified commit."""
  return shell_utils.run([GIT, 'show', commit, '--format=%h', '-s']).rstrip()

def GetRemoteMasterHash(git_url):
  return shell_utils.run([GIT, 'ls-remote', git_url, '--verify',
                          'refs/heads/master'])
