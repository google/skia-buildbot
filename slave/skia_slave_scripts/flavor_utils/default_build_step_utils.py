# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Base class for all slave-side build steps. """

import errno
import os
import shutil

from utils import file_utils
from utils import shell_utils


class BuildStepUtils:
  """ Utilities to be used by subclasses of BuildStep.

  The methods in this class define how certain high-level functions should work.
  Each build step flavor should correspond to a subclass of BuildStepUtils which
  may override any of these functions as appropriate for that flavor.

  For example, the AndroidBuildStepUtils will override the functions for copying
  files between the host and Android device, as well as the RunFlavoredCmd
  function, so that commands may be run through ADB. """

  def __init__(self, build_step_instance):
    self._step = build_step_instance

  def _PathToBinary(self, binary):
    """ Returns the path to the given built executable. """
    return os.path.join('out', self._step.configuration, binary)

  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStepUtils flavors. """
    shell_utils.Bash([self._PathToBinary(app)] + args)

  def ReadFileOnDevice(self, filepath):
    """ Read the contents of a file on the associated device. Subclasses should
    override this method with one appropriate for reading the contents of a file
    on the device side. """
    with open(filepath) as f:
      return f.read()

  def CopyDirectoryContentsToDevice(self, host_dir, device_dir):
    """ Copy the contents of a host-side directory to a clean directory on the
    device side. Subclasses should override this method with one appropriate for
    copying the contents of a host-side directory to a clean device-side
    directory."""
    # For "normal" builders who don't have an attached device, we expect
    # host_dir and device_dir to be the same.
    if host_dir != device_dir:
      raise ValueError('For builders who do not have attached devices, copying '
                       'from host to device is undefined and only allowed if '
                       'host_dir and device_dir are the same.')

  def CopyDirectoryContentsToHost(self, device_dir, host_dir):
    """ Copy the contents of a device-side directory to a clean directory on the
    host side. Subclasses should override this method with one appropriate for
    copying the contents of a device-side directory to a clean host-side
    directory."""
    # For "normal" builders who don't have an attached device, we expect
    # host_dir and device_dir to be the same.
    if host_dir != device_dir:
      raise ValueError('For builders who do not have attached devices, copying '
                       'from host to device is undefined and only allowed if '
                       'host_dir and device_dir are the same.')

  def PushFileToDevice(self, src, dst):
    """ Copy the a single file from path "src" on the host to path "dst" on
    the device.  If the host IS the device we are testing, it's just a filecopy.
    Subclasses should override this method with one appropriate for
    pushing the file to the device. """
    shutil.copy(src, dst)

  def DeviceListDir(self, directory):
    """ List the contents of a directory on the connected device. """
    return os.listdir(directory)

  def DevicePathExists(self, path):
    """ Like os.path.exists(), but for a path on the connected device. """
    return os.path.exists(path)

  def DevicePathJoin(self, *args):
    """ Like os.path.join(), but for paths that will target the connected
    device. """
    return os.sep.join(args)

  def CreateCleanDeviceDirectory(self, directory):
    """ Creates an empty directory on an attached device. Subclasses with
    attached devices should override. For builders with no attached device, just
    make sure that the directory exists, since we may want to keep data. """
    # TODO(borenet): This should actually clean the directory, but we don't
    # because we want to avoid deleting historical bench data which we might
    # need.
    try:
      os.makedirs(directory)
    except OSError as e:
      if e.errno != errno.EEXIST:
        raise

  def CreateCleanHostDirectory(self, directory):
    """ Creates an empty directory on the host. Can be overridden by subclasses,
    but that should not be necessary. """
    file_utils.CreateCleanLocalDir(directory)

  def Install(self):
    """ Install the Skia executables. """
    pass

  def Compile(self, target):
    """ Compile the Skia executables. """
    # TODO(borenet): It would be nice to increase code sharing here.
    if 'VS2012' in self._step.builder_name:
      os.environ['GYP_MSVS_VERSION'] = '2012'
    os.environ['GYP_DEFINES'] = self._step.args['gyp_defines']
    print 'GYP_DEFINES="%s"' % os.environ['GYP_DEFINES']
    make_cmd = 'make'
    if os.name == 'nt':
      make_cmd = 'make.bat'
    cmd = [make_cmd,
           target,
           'BUILDTYPE=%s' % self._step.configuration,
           ]
    cmd.extend(self._step.default_make_flags)
    cmd.extend(self._step.make_flags)
    shell_utils.Bash(cmd)
