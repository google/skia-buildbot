#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Merge Skia into Android."""


import sys

from build_step import BuildStep


class MergeIntoAndroid(BuildStep):
  """BuildStep which merges Skia into Android."""

  def _Run(self):
    print 'No-op for now...'


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(MergeIntoAndroid))
