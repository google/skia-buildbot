#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""This module contains utilities related to Google Storage manipulations."""

import os
import posixpath
import shutil
import tempfile
import time

from common import chromium_utils
from slave import slave_utils

import file_utils

TIMESTAMP_STARTED_FILENAME = 'TIMESTAMP_LAST_UPLOAD_STARTED'
TIMESTAMP_COMPLETED_FILENAME = 'TIMESTAMP_LAST_UPLOAD_COMPLETED'
LAST_REBASELINED_BY_FILENAME = 'LAST_REBASELINED_BY'

FILES_CHUNK = 500

def DeleteStorageObject(object_name):
  """Delete an object on Google Storage."""
  gsutil = slave_utils.GSUtilSetup()
  command = [gsutil]
  command.extend(['rm', '-R', object_name])
  print 'Running command: %s' % command
  chromium_utils.RunCommand(command)


def CopyStorageDirectory(src_dir, dest_dir, gs_acl='private'):
  """Copy a directory from/to Google Storage."""
  gsutil = slave_utils.GSUtilSetup()
  command = [gsutil]
  command.extend(['cp', '-a', gs_acl, '-R', src_dir, dest_dir])
  print 'Running command: %s' % command
  chromium_utils.RunCommand(command)


def MoveStorageDirectory(src_dir, dest_dir):
  """Move a directory on Google Storage."""
  gsutil = slave_utils.GSUtilSetup()
  command = [gsutil]
  command.extend(['mv', '-p', src_dir, dest_dir])
  print 'Running command: %s' % command
  chromium_utils.RunCommand(command)


def ListStorageDirectory(dest_gsbase, subdir):
  """List the contents of the specified Storage directory."""
  gsbase_subdir = posixpath.join(dest_gsbase, subdir)
  status, output_gsutil_ls = slave_utils.GSUtilListBucket(gsbase_subdir, [])
  if status != 0:
    raise Exception(
        'Could not list contents of %s in Google Storage!' % gsbase_subdir)

  gs_files = []
  for line in set(output_gsutil_ls.splitlines()):
    # Ignore lines with warnings and status messages.
    if line and line.startswith(gsbase_subdir) and line != gsbase_subdir:
      gs_files.append(line)
  return gs_files


def DoesStorageObjectExist(object_name):
  """Checks if an object exists on Google Storage.

  Returns True if it exists else returns False.
  """
  gsutil = slave_utils.GSUtilSetup()
  command = [gsutil]
  command.extend(['ls', object_name])
  print 'Running command: %s' % command
  return chromium_utils.RunCommand(command) == 0


def DownloadDirectoryContentsIfChanged(gs_base, gs_relative_dir, local_dir):
  """Compares the TIMESTAMP_LAST_UPLOAD_COMPLETED and downloads if different.

  The goal of DownloadDirectoryContentsIfChanged and
  UploadDirectoryContentsIfChanged is to attempt to replicate directory level
  rsync functionality to the Google Storage directories we care about.
  """
  if _AreTimeStampsEqual(gs_base, gs_relative_dir, local_dir):
    print '\n\n=======Local directory is current=======\n\n'
  else:
    file_utils.CreateCleanLocalDir(local_dir)
    gs_source = posixpath.join(gs_base, gs_relative_dir, '*')
    slave_utils.GSUtilDownloadFile(src=gs_source, dst=local_dir)
    if not _AreTimeStampsEqual(gs_base, gs_relative_dir, local_dir):
      raise Exception('Failed to download from GS: %s' % gs_source)


def _GetChunks(seq, n):
  """Yield successive n-sized chunks from the specified sequence."""
  for i in xrange(0, len(seq), n):
    yield seq[i:i+n]


def DeleteDirectoryContents(gs_base, gs_relative_dir, files_to_delete):
  """Deletes the specified files from the Google Storage Directory.

  Args:
    gs_base: str - The Google Storage base. Eg: gs://rmistry.
    gs_relative_dir: str - Relative directory to the Google Storage base.
    files_to_delete: Files that should be deleted from the Google Storage
        directory. The files are deleted one at a time. If files_to_delete is
        None or empty then all directory contents are deleted.
  """
  gs_dest = posixpath.join(gs_base, gs_relative_dir)
  if files_to_delete:
    for file_to_delete in files_to_delete:
      DeleteStorageObject(object_name=posixpath.join(gs_dest, file_to_delete))
  else:
    DeleteStorageObject(gs_dest)


def UploadDirectoryContentsIfChanged(
    gs_base, gs_relative_dir, gs_acl, local_dir, force_upload=False,
    upload_chunks=False, files_to_upload=None):
  """Compares the TIMESTAMP_LAST_UPLOAD_COMPLETED and uploads if different.

  Args:
    gs_base: str - The Google Storage base. Eg: gs://rmistry.
    gs_relative_dir: str - Relative directory to the Google Storage base.
    gs_acl: str - ACL to use when uploading to Google Storage.
    local_dir: str - The local directory to upload.
    force_upload: bool - Whether upload should be done regardless of timestamps
        matching or not.
    upload_chunks: bool - Whether upload should be done in chunks or in a single
        command.
    files_to_upload: str seq - Specific files that should be uploaded, if not
        specified then all files in local_dir are uploaded. If upload_chunks is
        True then files will be uploaded in chunks else they will be uploaded
        one at a time. The Google Storage directory is not cleaned before upload
        if files_to_upload is specified.

  The goal of DownloadDirectoryContentsIfChanged and
  UploadDirectoryContentsIfChanged is to attempt to replicate directory level
  rsync functionality to the Google Storage directories we care about.

  Returns True if contents were uploaded, else returns False.
  """
  if not force_upload and _AreTimeStampsEqual(gs_base, gs_relative_dir,
                                              local_dir):
    print '\n\n=======Local directory is current=======\n\n'
    return False
  else:
    local_src = os.path.join(local_dir, '*')
    gs_dest = posixpath.join(gs_base, gs_relative_dir)
    timestamp_value = time.time()

    if not files_to_upload:
      print '\n\n=======Delete Storage directory before uploading=======\n\n'
      DeleteStorageObject(gs_dest)

    print '\n\n=======Writing new TIMESTAMP_LAST_UPLOAD_STARTED=======\n\n'
    WriteTimeStampFile(
        timestamp_file_name=TIMESTAMP_STARTED_FILENAME,
        timestamp_value=timestamp_value, gs_base=gs_base,
        gs_relative_dir=gs_relative_dir, local_dir=local_dir, gs_acl=gs_acl)

    if upload_chunks:
      if files_to_upload:
        local_files = [
            os.path.join(local_dir, local_file)
            for local_file in files_to_upload]
      else:
        local_files = [
            os.path.join(local_dir, local_file)
            for local_file in os.listdir(local_dir)]
      for files_chunk in _GetChunks(local_files, FILES_CHUNK):
        gsutil = slave_utils.GSUtilSetup()
        command = [gsutil, 'cp'] + files_chunk + [gs_dest]
        if chromium_utils.RunCommand(command) != 0:
          raise Exception(
              'Could not upload the chunk to Google Storage! The chunk: %s'
              % files_chunk)
    else:
      if files_to_upload:
        for file_to_upload in files_to_upload:
          if slave_utils.GSUtilDownloadFile(
              src=os.path.join(local_dir, file_to_upload), dst=gs_dest) != 0:
            raise Exception(
                'Could not upload %s to Google Storage!' % file_to_upload)
      else:
        if slave_utils.GSUtilDownloadFile(src=local_src, dst=gs_dest) != 0:
          raise Exception('Could not upload %s to Google Storage!' % local_src)

    print '\n\n=======Writing new TIMESTAMP_LAST_UPLOAD_COMPLETED=======\n\n'
    WriteTimeStampFile(
        timestamp_file_name=TIMESTAMP_COMPLETED_FILENAME,
        timestamp_value=timestamp_value, gs_base=gs_base,
        gs_relative_dir=gs_relative_dir, local_dir=local_dir, gs_acl=gs_acl)
    return True


def _AreTimeStampsEqual(gs_base, gs_relative_dir, local_dir):
  """Compares the local TIMESTAMP with the TIMESTAMP from Google Storage."""

  local_timestamp_file = os.path.join(local_dir, TIMESTAMP_COMPLETED_FILENAME)
  # Make sure that the local TIMESTAMP file exists.
  if not os.path.exists(local_timestamp_file):
    return False

  # Get the timestamp file from Google Storage.
  src = posixpath.join(gs_base, gs_relative_dir, TIMESTAMP_COMPLETED_FILENAME)
  temp_file = tempfile.mkstemp()[1]
  slave_utils.GSUtilDownloadFile(src=src, dst=temp_file)

  local_file_obj = open(local_timestamp_file, 'r')
  storage_file_obj = open(temp_file, 'r')
  try:
    local_timestamp = local_file_obj.read().strip()
    storage_timestamp = storage_file_obj.read().strip()
    return local_timestamp == storage_timestamp
  finally:
    local_file_obj.close()
    storage_file_obj.close()


def ReadTimeStampFile(timestamp_file_name, gs_base, gs_relative_dir):
  """Reads the specified TIMESTAMP file from the specified GS dir.

  Returns 0 if the file is empty or does not exist.
  """
  src = posixpath.join(gs_base, gs_relative_dir, timestamp_file_name)
  temp_file = tempfile.mkstemp()[1]
  slave_utils.GSUtilDownloadFile(src=src, dst=temp_file)

  storage_file_obj = open(temp_file, 'r')
  try:
    timestamp_value = storage_file_obj.read().strip()
    return timestamp_value if timestamp_value else "0"
  finally:
    storage_file_obj.close()


def WriteTimeStampFile(
    timestamp_file_name, timestamp_value, gs_base=None, gs_relative_dir=None,
    gs_acl=None, local_dir=None):
  """Adds a timestamp file to a Google Storage and/or a Local Directory.
  
  If gs_base, gs_relative_dir and gs_acl are provided then the timestamp is
  written to Google Storage. If local_dir is provided then the timestamp is
  written to a local directory.
  """
  timestamp_file = os.path.join(tempfile.gettempdir(), timestamp_file_name)
  f = open(timestamp_file, 'w')
  try:
    f.write(str(timestamp_value))
  finally:
    f.close()
  if local_dir:
    shutil.copyfile(timestamp_file,
                    os.path.join(local_dir, timestamp_file_name))
  if gs_base and gs_relative_dir and gs_acl:
    slave_utils.GSUtilCopyFile(filename=timestamp_file, gs_base=gs_base,
                               subdir=gs_relative_dir, gs_acl=gs_acl)
