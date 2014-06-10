#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module gs_utils."""

import __builtin__
import misc
import os
import posixpath
import shell_utils
import shutil
import sys
import tempfile
import time

# Appending to PYTHONPATH to find common.
sys.path.append(os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot', 'scripts'))
sys.path.append(os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot', 'scripts', 'common'))
sys.path.append(os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot', 'site_config'))
sys.path.append(os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot', 'third_party',
                             'twisted_10_2'))


from slave import slave_utils
import gs_utils
import unittest


GSUTIL_LOCATION = slave_utils.GSUtilSetup()

TEST_TIMESTAMP = '1354128965'
TEST_TIMESTAMP_2 = '1354128985'


class TestGSUtils(unittest.TestCase):

  def setUp(self):
    self._expected_commands = []
    self._test_temp_file = None
    self._test_gs_base = None
    self._test_destdir = None
    self._test_gs_acl = None
    self._local_tempdir = tempfile.mkdtemp()

    def _MockCommand(command):
      self.assertEquals(self._expected_commands.pop(0), ' '.join(command))

    def _MockGSUtilFileCopy(filename, gs_base, subdir, gs_acl):
      self.assertEquals(self._test_temp_file, filename)
      self.assertEquals(self._test_gs_base, gs_base)
      self.assertEquals(self._test_destdir, subdir)
      self.assertEquals(self._test_gs_acl, gs_acl)

    def _MockGSUtilDownloadFile(src, dst):
      pass

    self._original_bash_run_command = shell_utils.run
    shell_utils.run = _MockCommand

    self._original_gsutil_file_copy = slave_utils.GSUtilCopyFile
    slave_utils.GSUtilCopyFile = _MockGSUtilFileCopy

    self._original_gsutil_download_file = slave_utils.GSUtilDownloadFile
    slave_utils.GSUtilDownloadFile = _MockGSUtilDownloadFile

    self._original_file = __builtin__.open

  def tearDown(self):
    self.assertEquals(len(self._expected_commands), 0)
    shell_utils.run = self._original_bash_run_command
    slave_utils.GSUtilCopyFile = self._original_gsutil_file_copy
    slave_utils.GSUtilDownloadFile = self._original_gsutil_download_file
    __builtin__.open = self._original_file
    shutil.rmtree(self._local_tempdir)

  def test_delete_storage_object(self):
    self._expected_commands = [('%s rm -R superman' % GSUTIL_LOCATION)]
    gs_utils.delete_storage_object('superman')

  def test_upload_file(self):
    self._expected_commands = [(
        '%s cp -a public /fake/local/src/path gs://fake/remote/dest/path' %
        GSUTIL_LOCATION)]
    gs_utils.upload_file(
        local_src_path='/fake/local/src/path',
        remote_dest_path='gs://fake/remote/dest/path',
        gs_acl='public')

  def test_upload_dir_contents_empty(self):
    self._expected_commands = []
    gs_utils.upload_dir_contents(
        local_src_dir=self._local_tempdir, remote_dest_dir='remote_dest_dir',
        gs_acl='public')

  def test_upload_dir_contents_one_file(self):
    """Upload src_dir containing one file, and no subdirs."""
    self._test_upload_dir_contents(filenames=['file1'])

  def test_upload_dir_contents_multiple_files(self):
    """Upload src_dir containing multiple files, and no subdirs."""
    self._test_upload_dir_contents(filenames=['file1', 'file2'])

  def _test_upload_dir_contents(self, filenames):
    """Helper function for upload_dir_contents() unittests.

    Args:
      filenames: basenames of files to create within local_src_dir
    """
    # Account for http://skbug.com/2658 ('gs_utils.upload_dir_contents()
    # adds an extra level of directories under remote_dest_dir')
    extra_dir_level = os.path.basename(self._local_tempdir)

    local_src_dir = self._local_tempdir
    remote_dest_dir = 'remote_dest_dir'
    for filename in filenames:
      self._expected_commands.append('%s cp -a public %s %s' % (
          GSUTIL_LOCATION,
          os.path.join(local_src_dir, filename),
          posixpath.join(remote_dest_dir, extra_dir_level, filename)))
      with open(os.path.join(local_src_dir, filename), 'w'):
        pass
    gs_utils.upload_dir_contents(
        local_src_dir=local_src_dir, remote_dest_dir=remote_dest_dir,
        gs_acl='public')

  def test_upload_dir_contents_one_dir(self):
    """Upload src_dir containing a subdir, which in turn contains files."""
    # Account for http://skbug.com/2658 ('gs_utils.upload_dir_contents()
    # adds an extra level of directories under remote_dest_dir')
    extra_dir_level = os.path.basename(self._local_tempdir)

    local_src_dir = self._local_tempdir
    remote_dest_dir = 'remote_dest_dir'
    subdir = 'subdir'
    os.mkdir(os.path.join(local_src_dir, subdir))
    for filename in ['file1', 'file2']:
      self._expected_commands.append('%s cp -a public %s %s' % (
          GSUTIL_LOCATION,
          os.path.join(local_src_dir, subdir, filename),
          posixpath.join(remote_dest_dir, extra_dir_level, subdir, filename)))
      with open(os.path.join(local_src_dir, subdir, filename), 'w'):
        pass
    gs_utils.upload_dir_contents(
        local_src_dir=local_src_dir, remote_dest_dir=remote_dest_dir,
        gs_acl='public')

  def test_download_dir_contents(self):
    self._expected_commands = [(
        '%s -m cp -R superman batman' % GSUTIL_LOCATION)]
    gs_utils.download_dir_contents('superman', 'batman')

  def test_copy_dir_contents(self):
    self._expected_commands = [(
        '%s -m cp -a public -R superman batman' % GSUTIL_LOCATION)]
    gs_utils.copy_dir_contents('superman', 'batman', 'public')

  def test_does_storage_object_exist(self):
    self._expected_commands = [('%s ls superman' % GSUTIL_LOCATION)]
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
        local_dir=self._local_tempdir)

  def test_AreTimeStampsEqual(self):
    self._test_gs_base = 'gs://test'
    self._test_destdir = 'testdir'
    local_dir = self._local_tempdir

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
