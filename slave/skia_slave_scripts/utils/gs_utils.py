#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities related to Google Storage manipulations."""

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
