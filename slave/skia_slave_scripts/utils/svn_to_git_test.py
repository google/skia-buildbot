#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Tests for svn_to_git module."""


import misc
import os
import shutil
import subprocess
import svn_to_git
import sys
import tempfile
import unittest

sys.path.append(os.path.join(misc.BUILDBOT_PATH))

from site_config import skia_vars


class TestSvnToGit(unittest.TestCase):
  def setUp(self):
    self._oldcwd = os.getcwd()
    self._checkout_path = tempfile.mkdtemp()
    os.chdir(self._checkout_path)
    subprocess.check_call(['git', 'clone',
                           skia_vars.GetGlobalVariable('skia_git_url'),
                           '--no-checkout'],
                          stdout=subprocess.PIPE,
                          stderr=subprocess.PIPE)
    os.chdir('skia')

  def testSomeRevisions(self):
    # Known to exist.
    known_revs = {
      '1': '586101c79b0490b50623e76c71a5fd67d8d92b08',
      '100': '98de2bdbd12a01aaf347ca2549801b5940613f3f',
      '1000': 'f6a7c1106e0b33e043a95c053d072fc6e9454a23',
      '10000': '9d5fedc5a6fb9476df0d7e5814c9c315b655d5c6',
      '14765': 'b0ce4b6fc8da4c3aa491fc43512e9187df1dfdae',
    }
    for svn_rev, git_hash in known_revs.iteritems():
      self.assertEqual(git_hash, svn_to_git.GitHashFromSvnRev(svn_rev))

    # Known not to exist.
    for rev in (0, 14595, 99999999999):
      self.assertRaises(svn_to_git.GitCommitNotFoundError,
                        svn_to_git.GitHashFromSvnRev, str(rev))

  def tearDown(self):
    os.chdir(self._oldcwd)
    shutil.rmtree(self._checkout_path)


if __name__ == '__main__':
  unittest.main()
