#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module file_utils."""

import file_utils
import os
import shutil
import unittest


class TestFileUtils(unittest.TestCase):

  def setUp(self):
    self._path_exists_ret = True
    self._path_exists_called = False
    self._rm_tree_called = False
    self._make_dirs_called = False

    def _MockPathExists(directory):
      self._path_exists_called = True
      return self._path_exists_ret

    def _MockRmTree(directory, onerror=None):
      self._rm_tree_called = True

    def _MockMakeDirs(directory):
      self._make_dirs_called = True

    self._original_exists = os.path.exists
    os.path.exists = _MockPathExists

    self._original_rmtree = shutil.rmtree
    shutil.rmtree = _MockRmTree

    self._original_makedirs = os.makedirs
    os.makedirs = _MockMakeDirs

  def tearDown(self):
    os.path.exists = self._original_exists
    shutil.rmtree = self._original_rmtree
    os.makedirs = self._original_makedirs

  def test_CreateCleanLocalDir_PathExists(self):
    file_utils.CreateCleanLocalDir('/tmp/test')
    self.assertTrue(self._path_exists_called)
    self.assertTrue(self._rm_tree_called)
    self.assertTrue(self._make_dirs_called)

  def test_CreateCleanLocalDir_PathDoesNotExists(self):
    self._path_exists_ret = False
    file_utils.CreateCleanLocalDir('/tmp/test')
    self.assertTrue(self._path_exists_called)
    self.assertFalse(self._rm_tree_called)
    self.assertTrue(self._make_dirs_called)


if __name__ == '__main__':
  unittest.main()
