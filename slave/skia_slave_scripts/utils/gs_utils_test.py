#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module gs_utils."""

import __builtin__
import os
import shell_utils
import sys
import tempfile
import time

# Appending to PYTHONPATH to find common.
buildbot_path = os.path.join(os.path.abspath(os.path.dirname(__file__)),
                             os.pardir, os.pardir, os.pardir)
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'scripts'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'scripts', 'common'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'site_config'))
sys.path.append(os.path.join(buildbot_path, 'third_party', 'chromium_buildbot',
                             'third_party', 'twisted_10_2'))


import chromium_utils
from slave import slave_utils
import gs_utils
import unittest


GSUTIL_LOCATION = slave_utils.GSUtilSetup()


TEST_TIMESTAMP = '1354128965'
TEST_TIMESTAMP_2 = '1354128985'


class TestGSUtils(unittest.TestCase):

  def setUp(self):
    self._expected_command = None
    self._test_temp_file = None
    self._test_gs_base = None
    self._test_destdir = None
    self._test_gs_acl = None

    def _MockCommand(command):
      self.assertEquals(self._expected_command, ' '.join(command))

    def _MockGSUtilFileCopy(filename, gs_base, subdir, gs_acl):
      self.assertEquals(self._test_temp_file, filename)
      self.assertEquals(self._test_gs_base, gs_base)
      self.assertEquals(self._test_destdir, subdir)
      self.assertEquals(self._test_gs_acl, gs_acl)

    def _MockGSUtilDownloadFile(src, dst):
      pass

    self._original_run_command = chromium_utils.RunCommand
    chromium_utils.RunCommand = _MockCommand

    self._original_bash_run_command = shell_utils.run
    shell_utils.run = _MockCommand

    self._original_gsutil_file_copy = slave_utils.GSUtilCopyFile
    slave_utils.GSUtilCopyFile = _MockGSUtilFileCopy
    
    self._original_gsutil_download_file = slave_utils.GSUtilDownloadFile
    slave_utils.GSUtilDownloadFile = _MockGSUtilDownloadFile

    self._original_file = __builtin__.open

  def tearDown(self):
    chromium_utils.RunCommand = self._original_run_command
    shell_utils.run = self._original_bash_run_command
    slave_utils.GSUtilCopyFile = self._original_gsutil_file_copy
    slave_utils.GSUtilDownloadFile = self._original_gsutil_download_file
    __builtin__.open = self._original_file

  def test_delete_storage_object(self):
    self._expected_command = ('%s rm -R superman' % GSUTIL_LOCATION)
    gs_utils.delete_storage_object('superman')

  def test_copy_storage_directory(self):
    self._expected_command = (
        '%s cp -a public -R superman batman' % GSUTIL_LOCATION)
    gs_utils.copy_storage_directory('superman', 'batman', 'public')

  def test_does_storage_object_exist(self):
    self._expected_command = ('%s ls superman' % GSUTIL_LOCATION)
    gs_utils.does_storage_object_exist('superman')

  def test_write_timestamp_file(self):
    self._test_temp_file = os.path.join(tempfile.gettempdir(), 'TIMESTAMP')
    self._test_gs_base = 'gs://test'
    self._test_destdir = 'testdir'
    self._test_gs_acl = 'private'
    gs_utils.write_timestamp_file(
        timestamp_file_name='TIMESTAMP',
        timestamp_value=time.time(),
        gs_base=self._test_gs_base,
        gs_relative_dir=self._test_destdir,
        gs_acl=self._test_gs_acl,
        local_dir=tempfile.mkdtemp())

  def test_AreTimeStampsEqual(self):
    self._test_gs_base = 'gs://test'
    self._test_destdir = 'testdir'

    local_dir = tempfile.mkdtemp()  

    class _MockFile():
      def __init__(self, name, attributes):
        self._name = name

      def readlines(self):
        return []

      def read(self, arg1=None):
        if self._name == os.path.join(tempfile.gettempdir(), 'TIMESTAMP'):
          return TEST_TIMESTAMP
        else:
          return TEST_TIMESTAMP_2

      def close(self):
        pass

      def __enter__(self):
        return self

      def __exit__(self, *args):
        pass

      def write(self, string):
        pass

    __builtin__.open = _MockFile

    # Will be false because the tmp directory will have no TIMESTAMP in it.
    # pylint: disable=W0212
    self.assertFalse(
        gs_utils._are_timestamps_equal(
            local_dir=local_dir,
            gs_base=self._test_gs_base,
            gs_relative_dir=self._test_destdir))
 
    self._test_temp_file = os.path.join(local_dir, 'TIMESTAMP')
  
    # Will be false because the timestamps are different.
    # pylint: disable=W0212
    self.assertFalse(
        gs_utils._are_timestamps_equal(
            local_dir=local_dir,
            gs_base=self._test_gs_base,
            gs_relative_dir=self._test_destdir))


if __name__ == '__main__':
  unittest.main()
