#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module write_json_summary.py"""

import filecmp
import os
import shutil
import tempfile
import unittest

import write_json_summary


class TestWriteJsonSummary(unittest.TestCase):

  def setUp(self):
    self._test_json_dir = os.path.join(
        os.path.dirname(os.path.realpath(__file__)), 'test_data')
    self._output_file_name = 'summary.json'
    self._gm_json_path = os.path.join(self._test_json_dir, 'gm_json_mock.py')
    self._imagediffdb_path = os.path.join(self._test_json_dir,
                                          'imagediffdb_mock.py')
    self._skpdiff_output_csv = os.path.join(self._test_json_dir, 'output.csv')
    self._actual_output_dir = tempfile.mkdtemp()
    self._actual_output_file_path = os.path.join(self._actual_output_dir,
                                                 self._output_file_name)
    self._gs_output_dir = 'gs://dummy-bucket/output-dir'
    self._gs_skp_dir = 'gs://dummy-bucket/skps'
    self._img_root = '/tmp/'
    self._nopatch_img_dir_name = 'nopatch'
    self._withpatch_img_dir_name = 'withpatch'
    self._slave_num = 1

  def tearDown(self):
    shutil.rmtree(self._actual_output_dir)

  def test_DifferentFiles(self):
    write_json_summary.WriteJsonSummary(
        img_root=self._img_root,
        nopatch_json=os.path.join(self._test_json_dir, 'output1.json'),
        nopatch_img_dir_name=self._nopatch_img_dir_name,
        withpatch_json=os.path.join(self._test_json_dir, 'output2.json'),
        withpatch_img_dir_name=self._withpatch_img_dir_name,
        output_file_path=self._actual_output_file_path,
        gs_output_dir=self._gs_output_dir,
        gs_skp_dir=self._gs_skp_dir,
        slave_num=self._slave_num,
        gm_json_path=self._gm_json_path,
        imagediffdb_path=self._imagediffdb_path,
        skpdiff_output_csv=self._skpdiff_output_csv)

    self.assertTrue(
        filecmp.cmp(os.path.join(self._test_json_dir, self._output_file_name),
                    self._actual_output_file_path))

  def test_NoDifferentFiles(self):
    write_json_summary.WriteJsonSummary(
        img_root=self._img_root,
        nopatch_json=os.path.join(self._test_json_dir, 'output1.json'),
        nopatch_img_dir_name=self._nopatch_img_dir_name,
        withpatch_json=os.path.join(self._test_json_dir, 'output1.json'),
        withpatch_img_dir_name=self._withpatch_img_dir_name,
        output_file_path=self._actual_output_file_path,
        gs_output_dir=self._gs_output_dir,
        gs_skp_dir=self._gs_skp_dir,
        slave_num=self._slave_num,
        gm_json_path=self._gm_json_path,
        imagediffdb_path=self._imagediffdb_path,
        skpdiff_output_csv=self._skpdiff_output_csv)

    self.assertFalse(os.path.isfile(self._actual_output_file_path))


if __name__ == '__main__':
  unittest.main()

