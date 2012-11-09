#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for module gs_utils."""

import os
import sys

# Appending to PYTHONPATH to find common.
sys.path.append(os.path.join(os.pardir, os.pardir, 'third_party',
                             'chromium_buildbot', 'scripts'))
sys.path.append(os.path.join(os.pardir, os.pardir, 'third_party',
                             'chromium_buildbot', 'site_config'))
from common import chromium_utils

import gs_utils
import unittest


GSUTIL_LOCATION = os.path.join(
    os.pardir, os.pardir, 'third_party', 'chromium_buildbot', 'scripts',
    'slave', 'gsutil')


class TestGSUtils(unittest.TestCase):

  def setUp(self):
    self._expected_command = None

    def _MockCommand(command):
      self.assertEquals(self._expected_command, ' '.join(command))

    self._original_run_command = chromium_utils.RunCommand
    chromium_utils.RunCommand = _MockCommand

  def tearDown(self):
    chromium_utils.RunCommand = self._original_run_command

  def test_DeleteStorageObject(self):
    self._expected_command = ('%s rm -R superman' % GSUTIL_LOCATION)
    gs_utils.DeleteStorageObject('superman')

  def testCopyStorageDirectory(self):
    self._expected_command = (
        '%s cp -a public -R superman batman' % GSUTIL_LOCATION)
    gs_utils.CopyStorageDirectory('superman', 'batman', 'public')

  def test_DoesStorageObjectExist(self):
    self._expected_command = ('%s ls superman' % GSUTIL_LOCATION)
    gs_utils.DoesStorageObjectExist('superman')


if __name__ == '__main__':
  unittest.main()
