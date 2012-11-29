#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities related to Google Storage manipulations."""

import os
import posixpath
import tempfile
import time

from common import chromium_utils
from slave import slave_utils


def DeleteStorageObject(object_name):
  """Delete an object on Google Storage."""
  gsutil = slave_utils.GSUtilSetup()
  command = [gsutil]
  command.extend(['rm', '-R', object_name])
  print 'Running command: %s' % command
  chromium_utils.RunCommand(command)


def CopyStorageDirectory(src_dir, dest_dir, gs_acl):
  """Copy a directory from/to Google Storage."""
  gsutil = slave_utils.GSUtilSetup()
  command = [gsutil]
  command.extend(['cp', '-a', gs_acl, '-R', src_dir, dest_dir])
  print 'Running command: %s' % command
  chromium_utils.RunCommand(command)
  

def DoesStorageObjectExist(object_name):
  """Checks if an object exists on Google Storage.

  Returns True if it exists else returns False.
  """
  gsutil = slave_utils.GSUtilSetup()
  command = [gsutil]
  command.extend(['ls', object_name])
  print 'Running command: %s' % command
  return chromium_utils.RunCommand(command) == 0


def WriteCurrentTimeStamp(gs_base, dest_dir, gs_acl):
  """Adds a TIMESTAMP file to a Google Storage directory.
  
  The goal of WriteCurrentTimeStamp and ReadTimeStamp is to attempt to replicate
  directory level rsync functionality to the Google Storage directories we care
  about.
  """
  timestamp_file = os.path.join(tempfile.gettempdir(), 'TIMESTAMP')
  f = open(timestamp_file, 'w')
  try:
    f.write(str(time.time()))
  finally:
    f.close()
  slave_utils.GSUtilCopyFile(filename=timestamp_file, gs_base=gs_base,
                             subdir=dest_dir, gs_acl=gs_acl)


def AreTimeStampsEqual(local_dir, gs_base, gs_relative_dir):
  """Compares the local TIMESTAMP with the TIMESTAMP from Google Storage.
  
  The goal of WriteCurrentTimeStamp and ReadTimeStamp is to attempt to replicate
  directory level rsync functionality to the Google Storage directories we care
  about.
  """

  local_timestamp_file = os.path.join(local_dir, 'TIMESTAMP')
  # Make sure that the local TIMESTAMP file exists.
  if not os.path.exists(local_timestamp_file):
    return False

  # Get the timestamp file from Google Storage.
  src = posixpath.join(gs_base, gs_relative_dir, 'TIMESTAMP')
  temp_file = os.path.join(tempfile.gettempdir(), 'TIMESTAMP')
  slave_utils.GSUtilDownloadFile(src=src, dst=temp_file)

  local_file_obj = open(local_timestamp_file, 'r')
  storage_file_obj = open(temp_file, 'r')
  try:
    local_timestamp = local_file_obj.read()
    storage_timestamp = storage_file_obj.read()
    return local_timestamp == storage_timestamp
  finally:
    local_file_obj.close()
    storage_file_obj.close()
