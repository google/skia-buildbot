#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility for creating a write-through and a read-through checksum cache."""

__author__ = 'Ravi Mistry'


import hashlib
import json
import os
import posixpath

# Add required paths to PYTHONPATH if running as a script.
if '__main__' == __name__:
  import sys
  sys.path.append(
      os.path.join(os.pardir, os.pardir, os.pardir, 'third_party',
                   'chromium_buildbot', 'scripts'))
  sys.path.append(
      os.path.join(os.pardir, os.pardir, os.pardir, 'third_party',
                   'chromium_buildbot', 'site_config'))

import gs_utils


HASHING_ALGORITHM = 'bytewise-md5'


def GetFileMD5(f, block_size=1**20):
  """Creates MD5 hash of the specified file in a memory efficient manner.

  Breaks the file into 1MB chunks (or a different specified size) because
  hashlib.md5 tries to fit the whole file in memory.
  """
  md5 = hashlib.md5()
  while True:
    data = f.read(block_size)
    if not data:
      break
    md5.update(data)
  return md5.hexdigest()


def _GetFilenamesToHashes(local_root):
  """Populates and returns a map of file names to their hash values.

  Each entry in the map will look like this:
    "amazon.skp" : {
        "bytewise-md5" : "a01234",
    },

  Params:
    local_root: str, the directory that contains the files we have to hash.
  """
  local_files_to_hash = {}
  for local_file in os.listdir(local_root):
    if not os.path.isfile(os.path.join(local_root, local_file)):
      # Ignore directories.
      continue
    local_file_path = os.path.join(local_root, local_file)
    file_checksum = GetFileMD5(open(local_file_path))
    local_files_to_hash[local_file] = {HASHING_ALGORITHM: file_checksum}
  return local_files_to_hash


def _GetRemoteFilesLookup(remote_root):
  """Populates and returns a map of remote files for fast lookup.

  Each entry in the map will look like this:
    "amazon.skp_a01234.skp" : 1,
  The value for every dictionary entry will be 1. This value is meaningless
  and is not actually used. The keys are important here and are used to test for
  file membership in the remote directory.

  Params:
    remote_root: str, the Google Storage root directory. The root directory
        contains a HASHING_ALGORITHM directory that contains the remote files.
  """
  # Get list of all remote files in Google Storage.
  try:
    remote_files = gs_utils.ListStorageDirectory(
        dest_gsbase=remote_root, subdir=HASHING_ALGORITHM)
  except Exception:
    # The directory does not exist yet.
    remote_files = []

  # Create a map from the remote files list for quicker lookup.
  remote_files_lookup = {}
  for remote_file in remote_files:
    # 'gsutil ls' returns the complete file path (not just the file names like
    # os.listdir. Put only the basenames in the map.
    remote_files_lookup[os.path.basename(remote_file)] = 1

  return remote_files_lookup


def WriteThroughCache(local_root, remote_root, output_json_path,
                      gs_acl='private'):
  """Writes local files to Google Storage if they do not exist there.

  The algorithm used by this function is:
  * Populate a dictionary of local files and their hash values. Each entry in
    the dictionary will look like this:
      "amazon.skp" : {
          "bytewise-md5" : "a01234",
      },
  * Get the list of files from the HASHING_ALGORITHM directory in remote_root.
  * If a local file and its hash value does not exist in Google Storage's
    remote_root/HASHING_ALGORITHM directory then copy the file over. The
    remote filename will be: filename.extention_hash.extension
  * Output the dictionary of local files and their hash values to the specified
    output_json_path using json.dump.

  Params:
    local_root: str, directory containing local files.
    remote_root: str, the Google Storage root directory. The root directory
        contains a HASHING_ALGORITHM directory that contains the remote files.
    output_json_path: str, complete file path of the JSON file this function
        will create with mappings of file names to their hash values.
    gs_acl: str, canned ACL to use when uploading to Google Storage. Default
        value is 'private'.
  """

  if not os.path.isdir(local_root):
    raise ValueError('%s does not exist!' % local_root)

  # Create a map of local files to their hashes.
  local_files_to_hash = _GetFilenamesToHashes(local_root)
  if not local_files_to_hash:
    print '%s has no files!' % local_root
    return

  # Create a map of remote files for quicker lookup.
  remote_files_lookup = _GetRemoteFilesLookup(remote_root)

  # Loop through all local files and check against the remote files lookup map
  # to see if they should be uploaded.
  for filename, hash_dict in local_files_to_hash.iteritems():
    # Construct the remote file name we will check the existence of.
    file_extension = filename.split('.')[-1]
    remote_filename = '%s_%s.%s' % (filename, hash_dict[HASHING_ALGORITHM],
                                    file_extension)
    if not remote_files_lookup.get(remote_filename):
      gs_utils.CopyStorageDirectory(
          src_dir=os.path.join(local_root, filename),
          dest_dir=posixpath.join(remote_root, HASHING_ALGORITHM,
                                  remote_filename),
          gs_acl=gs_acl)

  # Output a mapping of all local files to their hashes into a JSON file.
  json.dump(local_files_to_hash, open(output_json_path, 'w'), indent=4,
            sort_keys=True)


def ReadThroughCache(local_root, remote_root, input_json_path):
  """Downloads files from Google Storage if they do not exist locally.

  The algorithm used by this function is:
  * Create the local_root directory if it does not already exist.
  * Get a dictionary listing all the desired files to download, and the hash
    digest of each file indicating its latest contents, from input_json_path.
    Each entry in the dictionary will look like this:
      "amazon.skp" : {
          "bytewise-md5" : "a01234",
      },
  * Get the dictionary of file names to their hash values from the specified
    JSON doc. The dictionary will look like the one above.
  * Delete all local files which are not in the JSON dictionary.
  * Download all files which do not exist locally or which have different
    hashes.

  Params:
    local_root: str, directory containing local files.
    remote_root: str, the Google Storage root directory. The root directory
        contains a HASHING_ALGORITHM directory that contains the remote files.
    input_json_path: str, complete file path of the JSON file this function
        will read with mappings of file names to their hash values.
  """

  if not os.path.isdir(local_root):
    # Create the local_root directory.
    os.makedirs(local_root)

  # Create a map of local files to their hashes.
  local_files_to_hash = _GetFilenamesToHashes(local_root)

  # Get the dict of filename to hashes from the JSON file.
  json_dict = json.load(open(input_json_path))

  # Delete any files in local_root that are not in the json_dict.
  extra_files = set(local_files_to_hash) - set(json_dict)
  if extra_files:
    print 'Deleting all these extra files: %s' % extra_files
  for extra_file in extra_files:
    os.remove(os.path.join(local_root, extra_file))

  # Loop through files and hashes in the specified JSON file.
  for filename, hash_dict in json_dict.iteritems():
    file_exists_locally = local_files_to_hash.get(filename) != None
    files_are_equal = hash_dict == local_files_to_hash.get(filename)

    if file_exists_locally and files_are_equal:
      # The local and remote files are the same, no need to download.
      pass
    else:
      # Download the missing file from Google Storage.
      file_extension = filename.split('.')[-1]
      remote_filename = '%s_%s.%s' % (filename, hash_dict[HASHING_ALGORITHM],
                                      file_extension)
      gs_utils.CopyStorageDirectory(
          src_dir=posixpath.join(remote_root, HASHING_ALGORITHM,
                                 remote_filename),
          dest_dir=os.path.join(local_root, filename))


if '__main__' == __name__:
  WriteThroughCache(
      local_root='/tmp/local_root',
      remote_root='gs://chromium-skia-gm/test/checksum-test',
      output_json_path='/tmp/json_root/test.json')
  ReadThroughCache(
      local_root='/tmp/local_root2',
      remote_root='gs://chromium-skia-gm/test/checksum-test',
      input_json_path='/tmp/json_root/test.json')

