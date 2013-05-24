# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Subclass for all slave-side ChromeOS build steps. """


from build_step import BuildStep, DeviceDirs
from utils import gs_utils
from utils import ssh_utils
import os


class ChromeOSBuildStep(BuildStep):
  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                     ['skia_%s' % app] + args)

  def ReadFileOnDevice(self, filepath):
    """ Read the contents of a file on the device. """
    return ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                            ['cat', filepath], echo=False)

  def _RemoveDirectoryOnDevice(self, directory):
    """ Delete a directory on the device. """
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', directory])
    except Exception:
      pass
    if 'DIRECTORY_EXISTS' in ssh_utils.RunSSH(self._ssh_username,
        self._ssh_host, self._ssh_port, ['if', '[', '-d', directory, '];',
                                         'then', 'echo', 'DIRECTORY_EXISTS;',
                                         'fi']):
      raise Exception('Failed to remove %s' % directory)

  def _CreateDirectoryOnDevice(self, directory):
    """ Create a directory on the device. """
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                 ['mkdir', '-p', directory])

  def PushFileToDevice(self, src, dst):
    """ Copy a file to the device. """
    ssh_utils.PutSCP(src, dst, self._ssh_username, self._ssh_host,
                     self._ssh_port)

  def CreateCleanDirectory(self, directory):
    self._RemoveDirectoryOnDevice(directory)
    self._CreateDirectoryOnDevice(directory)

  def CopyDirectoryContentsToDevice(self, host_dir, device_dir):
    """ Copy the contents of a host-side directory to a clean directory on the
    device side.
    """
    self.CreateCleanDirectory(device_dir)
    file_list = os.listdir(host_dir)
    for f in file_list:
      if f == gs_utils.TIMESTAMP_COMPLETED_FILENAME:
        continue
      if os.path.isfile(os.path.join(host_dir, f)):
        self.PushFileToDevice(os.path.join(host_dir, f), device_dir)
    ts_filepath = os.path.join(host_dir, gs_utils.TIMESTAMP_COMPLETED_FILENAME)
    if os.path.isfile(ts_filepath):
      self.PushFileToDevice(ts_filepath, device_dir)

  def __init__(self, args, **kwargs):
    self._ssh_host = args['ssh_host']
    self._ssh_port = args['ssh_port']
    self._ssh_username = 'root'
    super(ChromeOSBuildStep, self).__init__(args=args, **kwargs)
    prefix = '/usr/local/skiabot/skia_'
    self._device_dirs = DeviceDirs(perf_data_dir=prefix + 'perf',
                                   gm_dir=prefix + 'gm',
                                   gm_expected_dir=prefix + 'gm_expected',
                                   resource_dir=prefix + 'resources',
                                   skp_dir=prefix + 'skp',
                                   skp_perf_dir=prefix + 'skp_perf',
                                   skp_out_dir=prefix + 'skp_out',
                                   tmp_dir=prefix + 'tmp_dir')
