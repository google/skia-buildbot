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
    self._render_pictures_args = '--test1=test --test2=test --test3'
    self._nopatch_gpu = 'False'
    self._withpatch_gpu = 'True'

  def tearDown(self):
    shutil.rmtree(self._actual_html_dir)

  def test_CombineJsonSummaries_WithDifferences(self):
    worker_name_to_info = json_summary_combiner.CombineJsonSummaries(
        os.path.join(self._test_data_dir, 'differences'))
    for worker_name, worker_info in worker_name_to_info.items():
      worker_num = worker_name[-1]
      file_count = 0
      for file_info in worker_info.failed_files:
        file_count += 1
        self.assertEquals(file_info.file_name,
                          'file%s_%s.png' % (worker_name, file_count))
        self.assertEquals(file_info.skp_location,
                          'http://storage.cloud.google.com/dummy-bucket/skps'
                          '/%s/file%s_.skp' % (worker_name, worker_name))
        self.assertEquals(file_info.num_pixels_differing,
                          int('%s%s1' % (worker_num, file_count)))
        self.assertEquals(file_info.percent_pixels_differing,
                          int('%s%s2' % (worker_num, file_count)))
        self.assertEquals(file_info.max_diff_per_channel,
                          int('%s%s4' % (worker_num, file_count)))

      self.assertEquals(
          worker_info.skps_location,
          'gs://dummy-bucket/skps/%s' % worker_name)
      self.assertEquals(
          worker_info.files_location_nopatch,
          'gs://dummy-bucket/output-dir/%s/nopatch-images' % worker_name)
      self.assertEquals(
          worker_info.files_location_diffs,
          'gs://dummy-bucket/output-dir/%s/diffs' % worker_name)
      self.assertEquals(
          worker_info.files_location_whitediffs,
          'gs://dummy-bucket/output-dir/%s/whitediffs' % worker_name)

  def test_CombineJsonSummaries_NoDifferences(self):
    worker_name_to_info = json_summary_combiner.CombineJsonSummaries(
        os.path.join(self._test_data_dir, 'no_output'))
    self.assertEquals(worker_name_to_info, {})

  def _get_test_worker_name_to_info(self):
    worker_name_to_info = {
        'worker1': json_summary_combiner.WorkerInfo(
            worker_name='worker1',
            failed_files=[
                json_summary_combiner.FileInfo(
                    'fileworker1_1.png',
                    'http://storage.cloud.google.com/dummy-bucket/skps/worker1/'
                    'fileworker1_.skp',
                    111, 112, 114, 115),
                json_summary_combiner.FileInfo(
                    'fileworker1_2.png',
                    'http://storage.cloud.google.com/dummy-bucket/skps/worker1/'
                    'fileworker1_.skp',
                    121, 122, 124, 125)],
            skps_location='gs://dummy-bucket/skps/worker1',
            files_location_diffs='gs://dummy-bucket/worker1/diffs',
            files_location_whitediffs='gs://dummy-bucket/worker1/whitediffs',
            files_location_nopatch='gs://dummy-bucket/worker1/nopatch',
            files_location_withpatch='gs://dummy-bucket/worker1/withpatch'),
        'worker2': json_summary_combiner.WorkerInfo(
            worker_name='worker2',
            failed_files=[
                json_summary_combiner.FileInfo(
                    'fileworker2_1.png',
                    'http://storage.cloud.google.com/dummy-bucket/skps/worker2/'
                    'fileworker2_.skp',
                    211, 212, 214, 215)],
            skps_location='gs://dummy-bucket/skps/worker2',
            files_location_diffs='gs://dummy-bucket/worker2/diffs',
            files_location_whitediffs='gs://dummy-bucket/worker2/whitediffs',
            files_location_nopatch='gs://dummy-bucket/worker2/nopatch',
            files_location_withpatch='gs://dummy-bucket/worker2/withpatch'),
        'worker3': json_summary_combiner.WorkerInfo(
            worker_name='worker3',
            failed_files=[
                json_summary_combiner.FileInfo(
                    'fileworker3_1.png',
                    'http://storage.cloud.google.com/dummy-bucket/skps/worker3/'
                    'fileworker3_.skp',
                    311, 312, 314, 315),
                json_summary_combiner.FileInfo(
                    'fileworker3_2.png',
                    'http://storage.cloud.google.com/dummy-bucket/skps/worker3/'
                    'fileworker3_.skp',
                    321, 322, 324, 325),
                json_summary_combiner.FileInfo(
                    'fileworker3_3.png',
                    'http://storage.cloud.google.com/dummy-bucket/skps/worker3/'
                    'fileworker3_.skp',
                    331, 332, 334, 335),
                json_summary_combiner.FileInfo(
                    'fileworker3_4.png',
                    'http://storage.cloud.google.com/dummy-bucket/skps/worker3/'
                    'fileworker3_.skp',
                    341, 342, 344, 345)],
            skps_location='gs://dummy-bucket/skps/worker3',
            files_location_diffs='gs://dummy-bucket/worker3/diffs',
            files_location_whitediffs='gs://dummy-bucket/worker3/whitediffs',
            files_location_nopatch='gs://dummy-bucket/worker3/nopatch',
            files_location_withpatch='gs://dummy-bucket/worker3/withpatch')
    }
    return worker_name_to_info

  def test_OutputToHTML_WithDifferences_WithAbsoluteUrl(self):
    worker_name_to_info = self._get_test_worker_name_to_info()
    json_summary_combiner.OutputToHTML(
        worker_name_to_info=worker_name_to_info,
        output_html_dir=self._actual_html_dir,
        absolute_url=self._absolute_url,
        render_pictures_args=self._render_pictures_args,
        nopatch_gpu=self._nopatch_gpu,
        withpatch_gpu=self._withpatch_gpu)

    html_expected_dir = os.path.join(self._test_data_dir, 'html_outputs',
                                     'differences_with_url')
    for html_file in ('index.html', 'list_of_all_files.html',
                      'fileworker1_1.png.html', 'fileworker1_2.png.html',
                      'fileworker2_1.png.html', 'fileworker3_1.png.html',
                      'fileworker3_2.png.html', 'fileworker3_3.png.html',
                      'fileworker3_4.png.html'):
      self.assertTrue(
          filecmp.cmp(os.path.join(html_expected_dir, html_file),
                      os.path.join(self._actual_html_dir, html_file)))

  def test_OutputToHTML_WithDifferences_WithNoUrl(self):
    worker_name_to_info = self._get_test_worker_name_to_info()
    json_summary_combiner.OutputToHTML(
        worker_name_to_info=worker_name_to_info,
        output_html_dir=self._actual_html_dir,
        absolute_url='',
        render_pictures_args=self._render_pictures_args,
        nopatch_gpu=self._nopatch_gpu,
        withpatch_gpu=self._withpatch_gpu)

    html_expected_dir = os.path.join(self._test_data_dir, 'html_outputs',
                                     'differences_no_url')
    for html_file in ('index.html', 'list_of_all_files.html',
                      'fileworker1_1.png.html', 'fileworker1_2.png.html',
                      'fileworker2_1.png.html', 'fileworker3_1.png.html',
                      'fileworker3_2.png.html', 'fileworker3_3.png.html',
                      'fileworker3_4.png.html'):
      self.assertTrue(
          filecmp.cmp(os.path.join(html_expected_dir, html_file),
                      os.path.join(self._actual_html_dir, html_file)))

  def test_OutputToHTML_NoDifferences(self):
    json_summary_combiner.OutputToHTML(
        worker_name_to_info={},
        output_html_dir=self._actual_html_dir,
        absolute_url='',
        render_pictures_args=self._render_pictures_args,
        nopatch_gpu=self._nopatch_gpu,
        withpatch_gpu=self._withpatch_gpu)

    html_expected_dir = os.path.join(self._test_data_dir, 'html_outputs',
                                     'nodifferences')
    self.assertTrue(
        filecmp.cmp(os.path.join(html_expected_dir, 'index.html'),
                    os.path.join(self._actual_html_dir, 'index.html')))


if __name__ == '__main__':
  unittest.main()
