#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Wrapper around common repo's gs_utils.py with buildbot-specific overrides."""

# System-level imports
import os
import sys

# Make sure the "common" repo is in PYTHON_PATH
_BUILDBOT_PATH = os.path.realpath(os.path.join(
    os.path.dirname(os.path.abspath(__file__)),
    os.pardir, os.pardir, os.pardir))
_COMMON_REPO_PATH = os.path.join(_BUILDBOT_PATH, 'common')
if not _COMMON_REPO_PATH in sys.path:
  sys.path.insert(0, _COMMON_REPO_PATH)

# Local imports
from py.utils import gs_utils as superclass_module

_DEFAULT_BOTO_FILE_PATH = os.path.join(
    _BUILDBOT_PATH, 'third_party', 'chromium_buildbot', 'site_config', '.boto')
_GS_PREFIX = 'gs://'


class GSUtils(superclass_module.GSUtils):
  """Wrapper around common repo's GSUtils with buildbot-specific overrides."""

  # The ACLs to use while copying playback (SKP) files to Google Storage.
  # They should not be world-readable!
  #
  # TODO(rmistry): Change "playback" variable names to something that makes more
  # sense to Eric.
  PLAYBACK_CANNED_ACL = superclass_module.GSUtils.PredefinedACL.PRIVATE
  PLAYBACK_FINEGRAINED_ACL_LIST = [
      (superclass_module.GSUtils.IdType.GROUP_BY_DOMAIN, 'google.com',
       superclass_module.GSUtils.Permission.READ),
  ]

  def __init__(self, boto_file_path=_DEFAULT_BOTO_FILE_PATH):
    """Override constructor to use buildbot credentials by default."""
    super(GSUtils, self).__init__(boto_file_path=boto_file_path)

  @staticmethod
  def with_gs_prefix(bucket_name):
    """Returns the bucket_name with _GS_PREFIX at the front.

    If _GS_PREFIX is already there, returns bucket_name as is.

    Examples:
      bucket1 -> gs://bucket1
      gs://bucket2 -> gs://bucket2
    """
    if bucket_name.startswith(_GS_PREFIX):
      return bucket_name
    else:
      return _GS_PREFIX + str(bucket_name)

  @staticmethod
  def without_gs_prefix(bucket_name):
    """Returns the bucket_name without _GS_PREFIX at the front.

    If bucket_name does not start with _GS_PREFIX, returns bucket_name as is.

    Examples:
      bucket1 -> bucket1
      gs://bucket2 -> bucket2
    """
    if bucket_name.startswith(_GS_PREFIX):
      return bucket_name[len(_GS_PREFIX):]
    else:
      return bucket_name
