# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" BuildStepUtils for a remote-execution-via-SSH environment. """


from default_build_step_utils import DefaultBuildStepUtils, DeviceDirs
from slave import slave_utils
from utils import gs_utils
from utils import shell_utils
from utils import ssh_utils
import os
import posixpath
import shutil


class SshBuildStepUtils(DefaultBuildStepUtils):
  def __init__(self, build_step_instance):
    DefaultBuildStepUtils.__init__(self, build_step_instance)
    self._unique_prefix = 'skia_'
    args = self._step.args
    self._ssh = ssh_utils.SshDestination(
      args.get('ssh_host', 'localhost'),
      args.get('ssh_port', 22),
      args.get('ssh_user', 'root'))
    if self._ssh.host == 'localhost':
      self._ssh.options = ['-o', 'NoHostAuthenticationForLocalhost=yes']

    self._build_dir = args.get('skia_out', os.path.abspath('out'))
    self._remote_dir = args.get('remote_dir', 'skia')

    # Subclass constructors can change these fields.
    #   self._unique_prefix - (string) prefix to add to target
    #                         executables.
    #   self._ssh.host - (string) hostname to pass to ssh
    #   self._ssh.port - (string) ssh port on the remote host
    #   self._ssh.user - (string) username on the remote host
    #   self._remote_dir - (string) absolute or relative path of a
    #                      directory on the remote system.
    #   self._build_dir - (string) This script assumes that the compiled
    #                      executables will be located in os.path.join(
    #                      self._build_dir, self._step.configuration)
    #
    # Subclasses should override Compile().  These values are
    # available if needed:
    #   self._step.configuration
    #   self._step.gyp_defines
    #   self._step.deps_target_os
    #   self._step.make_flags

  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    assert app in self.ListBuildStepExecutables()
    executable = self.DevicePathJoin(
      self._remote_dir, self._unique_prefix + app)
    self._ssh.Run([executable] + args)

  def ReadFileOnDevice(self, filepath):
    """ Read the contents of a file on the device. """
    return self._ssh.Run(['cat', filepath], echo=False)

  def _RemoveDirectoryOnDevice(self, directory):
    """ Delete a directory on the device. """
    try:
      self._ssh.Run(['rm', '-rf', directory])
    except shell_utils.CommandFailedException:
      # `rm -f` will return 0 for nonexistent operands. Other failurs
      # will return 1, leading to a CommandFailedException.
      raise Exception('Failed to remove %s' % directory)

  def _CreateDirectoryOnDevice(self, directory):
    """ Create a directory on the device. """
    self._ssh.Run(['mkdir', '-p', directory])

  def PushFileToDevice(self, src, dst):
    """ Overrides build_step.PushFileToDevice() """
    self._ssh.Put(src, dst)

  def PushMultipleFilesToDevice(self, src_files, dst):
    self._ssh.MultiPut(src_files, dst)

  def DeviceListDir(self, directory):
    """ Overrides build_step.DeviceListDir() """
    ls = self._ssh.Run(['ls', directory], echo=False)
    # ''.split('\n') evaluates to [''], when we want [].
    return [path for path in ls.split('\n') if path != '']

  def _RunAndReturnSuccess(self, command):
    try:
      self._ssh.Run(command)
    except shell_utils.CommandFailedException:
      return False
    return True

  def DevicePathExists(self, path):
    """ Overrides build_step.DevicePathExists() """
    return self._RunAndReturnSuccess(['test', '-e', path])

  def DeviceDirectoryExists(self, path):
    return self._RunAndReturnSuccess(['test', '-d', path])

  def DevicePathJoin(self, *args):
    """ Overrides build_step.DevicePathJoin() """
    return posixpath.sep.join(args)

  def CreateCleanDeviceDirectory(self, directory):
    # Raises exeption if either the directory exists but is
    # un-removeable or if mkdir fails.
    self._ssh.RunCmd('rm -rf "%s" && mkdir -p "%s"' % (directory, directory))

  def CopyDirectoryContentsToDevice(self, host_dir, device_dir):
    """ Copy the contents of a host-side directory to a clean
    directory on the device side.
    """
    self.CreateCleanDeviceDirectory(device_dir)
    upload_list = []
    for filename in os.listdir(host_dir):
      if filename != gs_utils.TIMESTAMP_COMPLETED_FILENAME:
        path = os.path.join(host_dir, filename)
        if os.path.isfile(path):
          upload_list.append(path)
    if upload_list:
      self.PushMultipleFilesToDevice(upload_list, device_dir)
    ts_filepath = os.path.join(host_dir, gs_utils.TIMESTAMP_COMPLETED_FILENAME)
    if os.path.isfile(ts_filepath):
      self.PushFileToDevice(ts_filepath, device_dir)

  def CopyDirectoryContentsToHost(self, device_dir, host_dir):
    """ Copy the contents of a device-side directory to a clean
    directory on the host side.
    """
    self.CreateCleanHostDirectory(host_dir)
    if not self.DeviceDirectoryExists(device_dir):
      print 'Device directory "%s" does not exist.' % device_dir
      return
    if not self.DeviceListDir(device_dir):
      print 'Device directory "%s" is empty.' % device_dir
      return
    self._ssh.Get(host_dir, posixpath.join(device_dir, '*'), recurse=True)

  def Install(self):
    """ Install the Skia executables. """
    for executable in self.ListBuildStepExecutables():
      # First, make sure that the program isn't running.
      remote_executable = self._unique_prefix + executable
      try:
        self._ssh.Run(['killall', remote_executable])
      except Exception:
        pass
      remote_path = self.DevicePathJoin(
        self._remote_dir, remote_executable)
      local_path = os.path.join(
        self._build_dir, self._step.configuration, executable)
      assert os.path.isfile(local_path)
      self._ssh.Put(local_path, remote_path)

  def AddGsutilToPath(self):
    # Add gsutil to PATH
    gsutil_dir = os.path.dirname(slave_utils.GSUtilSetup())
    if gsutil_dir not in os.environ['PATH'].split(os.pathsep):
      os.environ['PATH'] += os.pathsep + gsutil_dir

  def GetDeviceDirs(self):
    """ Set the directories which will be used by the BuildStep. """
    prefix = self.DevicePathJoin(self._remote_dir, self._unique_prefix)
    return DeviceDirs(
      perf_data_dir=prefix + 'perf',
      gm_actual_dir=prefix + 'gm_actual',
      gm_expected_dir=prefix + 'gm_expected',
      resource_dir=prefix + 'resources',
      skimage_in_dir=prefix + 'skimage_in',
      skimage_expected_dir=prefix + 'skimage_expected',
      skimage_out_dir=prefix + 'skimage_out',
      skp_dir=prefix + 'skp',
      skp_perf_dir=prefix + 'skp_perf',
      playback_actual_images_dir=prefix + 'playback_actual_images',
      playback_actual_summaries_dir=prefix + 'playback_actual_summaries',
      playback_expected_summaries_dir=prefix + 'playback_expected_summaries',
      tmp_dir=prefix + 'tmp_dir')

  def MakeClean(self):
    """ Overridden from DefaultBuildStepUtils """
    if os.path.isdir(self._build_dir):
      shutil.rmtree(os.path.realpath(self._build_dir))
