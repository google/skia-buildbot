# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for ChromeOS build steps. """


from default_build_step_utils import DefaultBuildStepUtils, DeviceDirs
from slave import slave_utils
from utils import gs_utils
from utils import shell_utils
from utils import ssh_utils
import os
import posixpath


class ChromeosBuildStepUtils(DefaultBuildStepUtils):
  def __init__(self, build_step_instance):
    DefaultBuildStepUtils.__init__(self, build_step_instance)
    self._ssh_host = self._step.args['ssh_host']
    self._ssh_port = self._step.args['ssh_port']
    self._ssh_username = 'root'

  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStep flavors. """
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host,
                     self._ssh_port, ['skia_%s' % app] + args)

  def ReadFileOnDevice(self, filepath):
    """ Read the contents of a file on the device. """
    return ssh_utils.RunSSH(self._ssh_username, self._ssh_host,
                            self._ssh_port, ['cat', filepath], echo=False)

  def _RemoveDirectoryOnDevice(self, directory):
    """ Delete a directory on the device. """
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host,
                       self._ssh_port, ['rm', '-rf', directory])
    except Exception:
      pass
    if self.DevicePathExists(directory):
      raise Exception('Failed to remove %s' % directory)

  def _CreateDirectoryOnDevice(self, directory):
    """ Create a directory on the device. """
    ssh_utils.RunSSH(self._ssh_username, self._ssh_host,
                     self._ssh_port, ['mkdir', '-p', directory])

  def PushFileToDevice(self, src, dst):
    """ Overrides build_step.PushFileToDevice() """
    ssh_utils.PutSCP(src, dst, self._ssh_username, self._ssh_host,
                     self._ssh_port)

  def DeviceListDir(self, directory):
    """ Overrides build_step.DeviceListDir() """
    return ssh_utils.RunSSH(
        self._ssh_username,
        self._ssh_host,
        self._ssh_port,
        ['ls', directory], echo=False).split('\n')

  def DevicePathExists(self, path):
    """ Overrides build_step.DevicePathExists() """
    return 'FILE_EXISTS' in ssh_utils.RunSSH(
        self._ssh_username,
        self._ssh_host,
        self._ssh_port,
        ['if', '[', '-e', path, '];', 'then', 'echo', 'FILE_EXISTS;', 'fi'])

  def DevicePathJoin(self, *args):
    """ Overrides build_step.DevicePathJoin() """
    return posixpath.sep.join(args)

  def CreateCleanDeviceDirectory(self, directory):
    self._RemoveDirectoryOnDevice(directory)
    self._CreateDirectoryOnDevice(directory)

  def CopyDirectoryContentsToDevice(self, host_dir, device_dir):
    """ Copy the contents of a host-side directory to a clean directory on the
    device side.
    """
    self.CreateCleanDeviceDirectory(device_dir)
    file_list = os.listdir(host_dir)
    for f in file_list:
      if f == gs_utils.TIMESTAMP_COMPLETED_FILENAME:
        continue
      if os.path.isfile(os.path.join(host_dir, f)):
        self.PushFileToDevice(os.path.join(host_dir, f), device_dir)
    ts_filepath = os.path.join(host_dir, gs_utils.TIMESTAMP_COMPLETED_FILENAME)
    if os.path.isfile(ts_filepath):
      self.PushFileToDevice(ts_filepath, device_dir)

  def CopyDirectoryContentsToHost(self, device_dir, host_dir):
    """ Copy the contents of a device-side directory to a clean directory on the
    host side.
    """
    self.CreateCleanHostDirectory(host_dir)
    if ssh_utils.RunSSH(self._ssh_username, self._ssh_host,
                        self._ssh_port, ['ls', device_dir]):
      ssh_utils.GetSCP(host_dir, posixpath.join(device_dir, '*'),
                       self._ssh_username, self._ssh_host,
                       self._ssh_port, recurse=True)

  def _PutSCP(self, executable):
    # First, make sure that the program isn't running.
    try:
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host,
                       self._ssh_port, ['killall', 'skia_%s' % executable])
    except Exception:
      pass
    ssh_utils.PutSCP(local_path=os.path.join('out', 'config',
                                             ('chromeos-' +
                                                  self._step.args['board']),
                                             self._step.configuration,
                                             executable),
                     remote_path='/usr/local/bin/skia_%s' % executable,
                     username=self._ssh_username,
                     host=self._ssh_host,
                     port=self._ssh_port)

  def Install(self):
    """ Install the Skia executables. """
    # TODO(borenet): Make it so that we don't have to list the executables here.
    self._PutSCP('tests')
    self._PutSCP('gm')
    self._PutSCP('render_pictures')
    self._PutSCP('render_pdfs')
    self._PutSCP('bench')
    self._PutSCP('bench_pictures')
    self._PutSCP('skimage')

  def Compile(self, target):
    """ Compile the Skia executables. """
    # Add gsutil to PATH
    gsutil = slave_utils.GSUtilSetup()
    os.environ['PATH'] += os.pathsep + os.path.dirname(gsutil)

    # Run the chromeos_make script.
    make_cmd = os.path.join('platform_tools', 'chromeos', 'bin',
                            'chromeos_make')
    cmd = [make_cmd,
           '-d', self._step.args['board'],
           target,
           'BUILDTYPE=%s' % self._step.configuration,
           ]

    cmd.extend(self._step.default_make_flags)
    cmd.extend(self._step.make_flags)
    shell_utils.run(cmd)

  def GetDeviceDirs(self):
    """ Set the directories which will be used by the BuildStep. """
    prefix = '/usr/local/skiabot/skia_'
    return DeviceDirs(perf_data_dir=prefix + 'perf',
                      gm_actual_dir=prefix + 'gm_actual',
                      gm_expected_dir=prefix + 'gm_expected',
                      resource_dir=prefix + 'resources',
                      skimage_in_dir=prefix + 'skimage_in',
                      skimage_expected_dir=prefix + 'skimage_expected',
                      skimage_out_dir=prefix + 'skimage_out',
                      skp_dir=prefix + 'skp',
                      skp_perf_dir=prefix + 'skp_perf',
                      skp_out_dir=prefix + 'skp_out',
                      tmp_dir=prefix + 'tmp_dir')
