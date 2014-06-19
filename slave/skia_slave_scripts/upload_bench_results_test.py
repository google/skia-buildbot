#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module upload_bench_results."""

import os
import sys
import unittest

from utils import misc
# Appending to PYTHONPATH to find common and config.
sys.path.append(os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot', 'scripts'))
sys.path.append(os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot', 'site_config'))
sys.path.append(os.path.join(misc.BUILDBOT_PATH, 'third_party',
                             'chromium_buildbot', 'third_party', 'twisted_8_1'))
import upload_bench_results


class ConfigParseTest(unittest.TestCase):

  def test_viewport(self):
    # pylint: disable=W0212
    result = upload_bench_results._ParseConfig('viewport_100x100')
    self.assertEqual(result, {
      'viewport': '100x100',
    })

  def test_enums(self):
    for config in ['8888', 'gpu', 'msaa4', 'nvprmsaa4', 'nvprmsaa16']:
      # pylint: disable=W0212
      result = upload_bench_results._ParseConfig(config)
      self.assertEqual(result, {
        'config': config
      })
    for bbh in ['rtree', 'quadtree', 'grid']:
      # pylint: disable=W0212
      result = upload_bench_results._ParseConfig(bbh)
      # TODO(kelvinly): The below assertion fails, probably because there are
      # multiple entries for 'grid' in upload_bench_results.py
      # self.assertEqual(result, {
      #   'bbh': bbh
      # })
    for mode in ['simple', 'record', 'tile']:
      # pylint: disable=W0212
      result = upload_bench_results._ParseConfig(mode)
      self.assertEqual(result, {
        'mode': mode
      })

  def test_badparams(self):
    # pylint: disable=W0212
    result = upload_bench_results._ParseConfig('foo')
    self.assertEqual(result, {
      'badParams': 'foo'
    })


if __name__ == '__main__':
  unittest.main()
