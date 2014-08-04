# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Utilities for generic build steps. """

import os
import shutil
import sys

from utils import file_utils
from py.utils import shell_utils


class DeviceDirs(object):
  def __init__(self, perf_data_dir, gm_actual_dir, gm_expected_dir,
               resource_dir, skimage_in_dir, skimage_expected_dir,
               skimage_out_dir, skp_dir, skp_perf_dir,
               playback_actual_images_dir, playback_actual_summaries_dir,
               playback_expected_summaries_dir, tmp_dir):
    self._perf_data_dir = perf_data_dir
    self._gm_actual_dir = gm_actual_dir
    self._gm_expected_dir = gm_expected_dir
    self._resource_dir = resource_dir
    self._skimage_in_dir = skimage_in_dir
    self._skimage_expected_dir = skimage_expected_dir
    self._skimage_out_dir = skimage_out_dir
    self._skp_dir = skp_dir
    self._skp_perf_dir = skp_perf_dir
    self._playback_actual_images_dir = playback_actual_images_dir
    self._playback_actual_summaries_dir = playback_actual_summaries_dir
    self._playback_expected_summaries_dir = playback_expected_summaries_dir
    self._tmp_dir = tmp_dir

  def GMActualDir(self):
    """Holds images and JSON summary written out by the 'gm' tool."""
    return self._gm_actual_dir

  def GMExpectedDir(self):
    """Holds expectations JSON summary read by the 'gm' tool."""
    return self._gm_expected_dir

  def PerfDir(self):
    return self._perf_data_dir

  def PlaybackActualImagesDir(self):
    """Holds image files written out by the 'render_pictures' tool."""
    return self._playback_actual_images_dir

  def PlaybackActualSummariesDir(self):
    """Holds actual-result JSON summaries written by 'render_pictures' tool."""
    return self._playback_actual_summaries_dir

  def PlaybackExpectedSummariesDir(self):
    """Holds expected-result JSON summaries read by 'render_pictures' tool."""
    return self._playback_expected_summaries_dir

  def ResourceDir(self):
    return self._resource_dir

  def SKImageInDir(self):
    return self._skimage_in_dir

  def SKImageExpectedDir(self):
    return self._skimage_expected_dir

  def SKImageOutDir(self):
    return self._skimage_out_dir

  def SKPDir(self):
    """Holds SKP files that are consumed by RenderSKPs and BenchPictures."""
    return self._skp_dir

  def SKPPerfDir(self):
    return self._skp_perf_dir

  def TmpDir(self):
    return self._tmp_dir


class DefaultBuildStepUtils:
  """ Utilities to be used by subclasses of BuildStep.

  The methods in this class define how certain high-level functions should work.
  Each build step flavor should correspond to a subclass of BuildStepUtils which
  may override any of these functions as appropriate for that flavor.

  For example, the AndroidBuildStepUtils will override the functions for copying
  files between the host and Android device, as well as the RunFlavoredCmd
  function, so that commands may be run through ADB. """

  def __init__(self, build_step_instance):
    self._step = build_step_instance

  def ListBuildStepExecutables(self):
    """ Called by subclasses that may need to install the executables. """
    return ['dm', 'gm', 'render_pictures', 'render_pdfs',
            'bench', 'bench_pictures', 'skimage', 'nanobench']

  def _PathToBinary(self, binary):
    """ Returns the path to the given built executable. """
    return os.path.join('out', self._step.configuration, binary)

  def RunFlavoredCmd(self, app, args):
    """ Override this in new BuildStepUtils flavors. """
    if (sys.platform == 'linux2' and 'x86_64' in self._step.builder_name
        and not 'TSAN' in self._step.builder_name):
      cmd = ['catchsegv', self._PathToBinary(app)]
    else:
      cmd = [self._PathToBinary(app)]
    shell_utils.run(cmd + args)

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
    directory.

    TODO(epoger): Clarify the description a bit: this method does not expect
    device_dir to be an empty directory before it is called.  Implementations
    of this method for other device types create an empty directory at
    device_dir as the first step.
    """
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
    attached devices should override. """
    file_utils.create_clean_local_dir(directory)

  def CreateCleanHostDirectory(self, directory):
    """ Creates an empty directory on the host. Can be overridden by subclasses,
    but that should not be necessary. """
    file_utils.create_clean_local_dir(directory)

  def Install(self):
    """ Install the Skia executables. """
    pass

  def RunGYP(self):
    self.Compile('gyp')

  def Compile(self, target):
    """ Compile the Skia executables. """
    # TODO(borenet): It would be nice to increase code sharing here.
    if 'Win8' in self._step.builder_name:
      os.environ['GYP_MSVS_VERSION'] = '2012'
      print 'GYP_MSVS_VERSION="%s"' % os.environ['GYP_MSVS_VERSION']

    os.environ['CHROME_PATH'] = os.path.join(os.path.expanduser('~'), 'src')
    print 'CHROME_PATH="%s"' % os.environ['CHROME_PATH']

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

    # TODO(epoger): Maybe remove this once we fix the underlying problem in
    # https://code.google.com/p/skia/issues/detail?id=2393 ('recurring RunGYP
    # failures on multiple Test-Win7-ShuttleA-HD2000-* bots')
    print 'about to run cmd %s' % cmd
    cwd = os.getcwd()
    print 'cwd is %s' % cwd
    print 'contents of cwd are %s' % os.listdir(cwd)

    shell_utils.run(cmd)

  def MakeClean(self):
    make_cmd = 'make'
    if os.name == 'nt':
      make_cmd = 'make.bat'
    shell_utils.run([make_cmd, 'clean'])

  def PreRun(self):
    """ Preprocessing step to run before the BuildStep itself. """
    pass

  def GetDeviceDirs(self):
    """ Set the directories which will be used by the BuildStep.

    In the case of DefaultBuildStepUtils, host_dirs and device_dirs are the
    same, which is why CopyDirectoryContentsToDevice() is a no-op.
    """
    return DeviceDirs(
        perf_data_dir=self._step.perf_data_dir,
        # TODO(epoger): Instead, set gm_actual_dir to self._step._gm_actual_dir
        # and remove os.path.join() with self._builder_name in postrender.py ?
        # (CopyDirectoryContentsToHost fails if the paths are different when
        # host==device, so why not just make them inherently equal?)
        gm_actual_dir=os.path.join(os.pardir, os.pardir, 'gm', 'actual'),
        gm_expected_dir=os.path.join(os.pardir, os.pardir, 'gm', 'expected'),
        resource_dir=self._step.resource_dir,
        skimage_in_dir=self._step.skimage_in_dir,
        skimage_expected_dir=os.path.join(os.pardir, os.pardir, 'skimage',
                                          'expected'),
        skimage_out_dir=self._step.skimage_out_dir,
        skp_dir=self._step.skp_dir,
        skp_perf_dir=self._step.perf_data_dir,
        playback_actual_images_dir=self._step.playback_actual_images_dir,
        playback_actual_summaries_dir=self._step.playback_actual_summaries_dir,
        playback_expected_summaries_dir=(
            self._step.playback_expected_summaries_dir),
        tmp_dir=os.path.join(os.pardir, 'tmp'))
