# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Utility class to build the Skia master BuildFactory's for ChromeOS buildbots.

Overrides SkiaFactory with any ChromeOS-specific steps."""


from skia_master_scripts import factory as skia_factory
from buildbot.process.properties import WithProperties


class ChromeOSFactory(skia_factory.SkiaFactory):
  """Overrides for ChromeOS builds."""

  def __init__(self, **kwargs):
    """ Instantiates a ChromeOSFactory with properties and build steps specific
    to ChromeOS devices.

    ssh_host: string indicating hostname or ip address of the target device
    ssh_port: string indicating the ssh port on the target device
    """
    skia_factory.SkiaFactory.__init__(self, flavor='chromeos',
                                      bench_pictures_cfg='no_gpu', **kwargs)
    self._common_args += ['--ssh_host', WithProperties('%(ssh_host:-None)s'),
                          '--ssh_port', WithProperties('%(ssh_port:-None)s')]
