# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import os

from utils import shell_utils
from utils import ssh_utils
from flavor_utils.ssh_build_step_utils import SshBuildStepUtils

class Arm64ModelBuildStepUtils(SshBuildStepUtils):
  # make sure deps_target_os = 'barelinux' so that all dependencies
  # can be installed.
  def __init__(self, build_step_instance):
    SshBuildStepUtils.__init__(self, build_step_instance)

    self._remote_dir = 'skia'
    self._build_dir = os.path.join('out', 'config', 'arm64linux')

    self._working_dir = os.path.abspath(self._step.args.get(
        'working_dir', os.path.join(os.pardir, os.pardir, 'arm64bareLinux')))

    key_file = os.path.join(self._working_dir, 'key')
    if os.path.isfile(key_file):
      ssh_utils.SSHAdd(key_file)

  def RunGYP(self):
    # barelinux_make handles gyp
    pass

  def Compile(self, target):
    platform_bin = os.path.join('platform_tools', 'barelinux', 'bin')
    # If working_dir doesn't exist, arm64_download will create it.
    # this script should download everything we need to start the
    # virtual machine, and then boot it up.  If it fails it will
    # return a non-zero exit status and shell_utils.run will throw an
    # exception.  We do not catch this exception.
    print 'Installing build tools and VM to', self._working_dir
    self.AddGsutilToPath()  # needed by arm64_download
    shell_utils.run(
      [os.path.join(platform_bin, 'arm64_download'), self._working_dir])

    assert os.path.isdir(self._working_dir)

    toolchain_bin = os.path.join(
      self._working_dir,
      'gcc-linaro-aarch64-linux-gnu-4.8-2013.12_linux',
      'bin')
    assert os.path.isdir(toolchain_bin)

    key_file = os.path.join(self._working_dir, 'key')
    assert os.path.isfile(key_file)
    ssh_utils.SSHAdd(key_file)

    platform_bin = os.path.join('platform_tools', 'barelinux', 'bin')
    make_cmd = [
      os.path.join(platform_bin, 'barelinux_make'),
      '-o', self._build_dir,
      '-c', os.path.join(toolchain_bin, 'aarch64-linux-gnu-gcc'),
      '-x', os.path.join(toolchain_bin, 'aarch64-linux-gnu-g++'),
      '-t', self._step.configuration,
      'skia_gpu=0 skia_arch_type=arm skia_arch_width=64 '
      ' armv7=1 armv8=1 arm_neon=0 arm_thumb=0'
      ]
    shell_utils.run(make_cmd, log_in_real_time=False)
