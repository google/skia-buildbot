#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module write_json_summary.py"""

import filecmp
import io
import json
import os
import shutil
import sys
import tempfile
import unittest

PARENT_DIR = os.path.dirname(os.path.realpath(__file__))

import write_json_summary


class TestWriteJsonSummary(unittest.TestCase):

  def setUp(self):
    self.longMessage = True
    self.maxDiff = None

    self._test_json_dir = os.path.join(PARENT_DIR, 'test_data')
    self._mocks_dir = os.path.join(PARENT_DIR, 'mocks')
    self._output_file_name = 'summary.json'
    self._actual_output_dir = tempfile.mkdtemp()
    self._actual_output_file_path = os.path.join(self._actual_output_dir,
                                                 self._output_file_name)
    self._gs_output_dir = 'gs://dummy-bucket/output-dir'
    self._gs_skp_dir = 'gs://dummy-bucket/skps'
    self._img_root = '/tmp/'
    self._nopatch_images_base_url = 'file://fake/path/to/nopatch'
    self._withpatch_images_base_url = 'file://fake/path/to/withpatch'
    self._slave_num = 1

  def tearDown(self):
    shutil.rmtree(self._actual_output_dir)

  def assertJsonFilesEqual(self, expected, actual):
    """Assert contents of two JSON files are equal, displaying any diffs.

    Args:
      expected: (str) Path to JSON file with desired content.
      actual: (str) Path to JSON file to evaluate.
    """
    if filecmp.cmp(expected, actual):
      return
    with io.open(expected, mode='r') as expected_filehandle:
      expected_dict = json.load(expected_filehandle)
    with io.open(actual, mode='r') as actual_filehandle:
      actual_dict = json.load(actual_filehandle)
    self.assertEqual(expected_dict, actual_dict, msg=(
        '\n\nexpectation (%s) differed from actual (%s)' % (expected, actual)))

  def test_DifferentFiles(self):
    write_json_summary.WriteJsonSummary(
        img_root=self._img_root,
        nopatch_json=os.path.join(self._test_json_dir, 'output1.json'),
        nopatch_images_base_url=self._nopatch_images_base_url,
        withpatch_json=os.path.join(self._test_json_dir, 'output2.json'),
        withpatch_images_base_url=self._withpatch_images_base_url,
        output_file_path=self._actual_output_file_path,
        gs_output_dir=self._gs_output_dir,
        gs_skp_dir=self._gs_skp_dir,
        slave_num=self._slave_num,
        additions_to_sys_path=[self._mocks_dir])
    self.assertJsonFilesEqual(
        expected=os.path.join(self._test_json_dir, self._output_file_name),
        actual=self._actual_output_file_path)

  def test_NoDifferentFiles(self):
    write_json_summary.WriteJsonSummary(
        img_root=self._img_root,
        nopatch_json=os.path.join(self._test_json_dir, 'output1.json'),
        nopatch_images_base_url=self._nopatch_images_base_url,
        withpatch_json=os.path.join(self._test_json_dir, 'output1.json'),
        withpatch_images_base_url=self._withpatch_images_base_url,
        output_file_path=self._actual_output_file_path,
        gs_output_dir=self._gs_output_dir,
        gs_skp_dir=self._gs_skp_dir,
        slave_num=self._slave_num,
        additions_to_sys_path=[self._mocks_dir])
    self.assertFalse(os.path.isfile(self._actual_output_file_path))


if __name__ == '__main__':
  unittest.main()
