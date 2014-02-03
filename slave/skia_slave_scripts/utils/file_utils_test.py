#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module file_utils."""

import file_utils
import os
import unittest

from common import chromium_utils

class TestFileUtils(unittest.TestCase):

  def setUp(self):
    self._path_exists_ret = True
    self._path_exists_called = False
    self._make_dirs_called = False
    self._remove_dir_called = False

    def _MockPathExists(directory):
      self._path_exists_called = True
      return self._path_exists_ret

    def _MockMakeDirs(directory):
      self._make_dirs_called = True

    def _MockRemoveDirectory(directory):
      self._remove_dir_called = True

    self._original_exists = os.path.exists
    os.path.exists = _MockPathExists

    self._original_makedirs = os.makedirs
    os.makedirs = _MockMakeDirs

    self._original_remove_dir = chromium_utils.RemoveDirectory
    chromium_utils.RemoveDirectory = _MockRemoveDirectory

  def tearDown(self):
    os.path.exists = self._original_exists
    os.makedirs = self._original_makedirs
    chromium_utils.RemoveDirectory = self._original_remove_dir

  def test_create_clean_local_dir_PathExists(self):
    file_utils.create_clean_local_dir('/tmp/test')
    self.assertTrue(self._path_exists_called)
    self.assertTrue(self._make_dirs_called)
    self.assertTrue(self._remove_dir_called)

  def test_create_clean_local_dir_PathDoesNotExists(self):
    self._path_exists_ret = False
    file_utils.create_clean_local_dir('/tmp/test')
    self.assertTrue(self._path_exists_called)
    self.assertTrue(self._make_dirs_called)
    self.assertFalse(self._remove_dir_called)


if __name__ == '__main__':
  unittest.main()
