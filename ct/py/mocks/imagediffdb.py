#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Dummy module that pretends to be imagediffdb.py for write_json_summary_test.

imagediffdb.py here refers to
https://code.google.com/p/skia/source/browse/trunk/gm/rebaseline_server/imagediffdb.py

TODO(rmistry): As noted in https://codereview.chromium.org/183763025 ,
it would be good for us to add lots of assertions about how this mock gets
called during unittests.
"""


class DiffRecord(object):

  def __init__(self, storage_root=None, expected_image_url=None,
               expected_image_locator=None, actual_image_url=None,
               actual_image_locator=None, expected_images_subdir=None,
               actual_images_subdir=None):
    pass

  def get_num_pixels_differing(self):
    return 1

  def get_percent_pixels_differing(self):
    return 2

  def get_max_diff_per_channel(self):
    return 4

  def get_perceptual_difference(self):
    return 5


class ImageDiffDB(object):

  def __init__(self, storage_root):
    pass

  def add_image_pair(self, expected_image_url, expected_image_locator,
                     actual_image_url, actual_image_locator):
    pass

  def get_diff_record(self, expected_image_locator, actual_image_locator):
    return DiffRecord()
