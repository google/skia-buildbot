# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Utility class to build the Skia master BuildFactory's for
Arm64BareLinux buildbots.

Overrides SkiaFactory with any specific steps."""


from skia_master_scripts import factory as skia_factory
from buildbot.process.properties import WithProperties


class Arm64ModelFactory(skia_factory.SkiaFactory):
  """Overrides for Arm64Model builds."""

  def __init__(self, board, **kwargs):
    """ Instantiates a Arm64ModelFactory with properties and build
    steps specific to Arm64ModelFactory devices.

    ssh_host: string indicating hostname or ip address of the target device
    ssh_port: string indicating the ssh port on the target device
    ssh_user: string indicating the login on the target device
    """
    skia_factory.SkiaFactory.__init__(
      self, flavor='arm64model', deps_target_os='barelinux', **kwargs)
    self._common_args += [
      '--ssh_host', WithProperties('%(ssh_host:-localhost)s'),
      '--ssh_port', WithProperties('%(ssh_port:-8022)s'),
      '--ssh_user', WithProperties('%(ssh_user:-user)s'),
      ]

