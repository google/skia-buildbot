#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module json_summary_combiner.py"""

import filecmp
import os
import shutil
import tempfile
import unittest

import json_summary_combiner


class TestJsonSummaryCombiner(unittest.TestCase):

  def setUp(self):
    self._test_data_dir = os.path.join(
        os.path.dirname(os.path.realpath(__file__)), 'test_data', 'combiner')
    self._actual_html_dir = tempfile.mkdtemp()
    self._absolute_url = 'http://dummy-link.foobar/'

  def tearDown(self):
    shutil.rmtree(self._actual_html_dir)

  def test_CombineJsonSummaries_WithDifferences(self):
    slave_name_to_info = json_summary_combiner.CombineJsonSummaries(
        os.path.join(self._test_data_dir, 'differences'))
    for slave_name, slave_info in slave_name_to_info.items():
      self.assertEquals(
          slave_info.failed_files,
          ['file%s-1.png' % slave_name, 'file%s-2.png' % slave_name])
      self.assertEquals(
          slave_info.skps_location,
          'gs://dummy-bucket/skps/%s' % slave_name)
      self.assertEquals(
          slave_info.files_location_nopatch,
          'gs://dummy-bucket/output-dir/%s/nopatch-images' % slave_name)
      self.assertEquals(
          slave_info.files_location_withpatch,
          'gs://dummy-bucket/output-dir/%s/withpatch-images' % slave_name)

  def test_CombineJsonSummaries_NoDifferences(self):
    slave_name_to_info = json_summary_combiner.CombineJsonSummaries(
        os.path.join(self._test_data_dir, 'no_output'))
    self.assertEquals(slave_name_to_info, {})

  def _get_test_slave_name_to_info(self):
    slave_name_to_info = {
        'slave1': json_summary_combiner.SlaveInfo(
            slave_name='slave1',
            failed_files=['fileslave1-1.png', 'fileslave1-2.png'],
            skps_location='gs://dummy-bucket/skps/slave1',
            files_location_nopatch='gs://dummy-bucket/slave1/nopatch',
            files_location_withpatch='gs://dummy-bucket/slave1/withpatch'),
        'slave2': json_summary_combiner.SlaveInfo(
            slave_name='slave2',
            failed_files=['fileslave2-1.png'],
            skps_location='gs://dummy-bucket/skps/slave2',
            files_location_nopatch='gs://dummy-bucket/slave2/nopatch',
            files_location_withpatch='gs://dummy-bucket/slave2/withpatch'),
        'slave3': json_summary_combiner.SlaveInfo(
            slave_name='slave3',
            failed_files=['fileslave3-1.png', 'fileslave3-2.png',
                          'fileslave3-3.png', 'fileslave3-4.png'],
            skps_location='gs://dummy-bucket/skps/slave3',
            files_location_nopatch='gs://dummy-bucket/slave3/nopatch',
            files_location_withpatch='gs://dummy-bucket/slave3/withpatch'),
    }
    return slave_name_to_info

  def test_OutputToHTML_WithDifferences_WithAbsoluteUrl(self):
    slave_name_to_info = self._get_test_slave_name_to_info()
    json_summary_combiner.OutputToHTML(
        slave_name_to_info=slave_name_to_info,
        output_html_dir=self._actual_html_dir,
        absolute_url=self._absolute_url)

    html_expected_dir = os.path.join(self._test_data_dir, 'html_outputs',
                                     'differences_with_url')
    for html_file in ('index.html', 'slave1.html', 'slave2.html',
                      'slave3.html'):
      self.assertTrue(
          filecmp.cmp(os.path.join(html_expected_dir, html_file),
                      os.path.join(self._actual_html_dir, html_file)))

  def test_OutputToHTML_WithDifferences_WithNoUrl(self):
    slave_name_to_info = self._get_test_slave_name_to_info()
    json_summary_combiner.OutputToHTML(
        slave_name_to_info=slave_name_to_info,
        output_html_dir=self._actual_html_dir,
        absolute_url='')

    html_expected_dir = os.path.join(self._test_data_dir, 'html_outputs',
                                     'differences_no_url')
    for html_file in ('index.html', 'slave1.html', 'slave2.html',
                      'slave3.html'):
      self.assertTrue(
          filecmp.cmp(os.path.join(html_expected_dir, html_file),
                      os.path.join(self._actual_html_dir, html_file)))

  def test_OutputToHTML_NoDifferences(self):
    json_summary_combiner.OutputToHTML(
        slave_name_to_info={},
        output_html_dir=self._actual_html_dir,
        absolute_url='')

    html_expected_dir = os.path.join(self._test_data_dir, 'html_outputs',
                                     'nodifferences')
    self.assertTrue(
        filecmp.cmp(os.path.join(html_expected_dir, 'index.html'),
                    os.path.join(self._actual_html_dir, 'index.html')))


if __name__ == '__main__':
  unittest.main()

