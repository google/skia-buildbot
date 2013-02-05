# Copyright (c) 2012 The Chromium Authors. All rights reserved.
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
    skia_factory.SkiaFactory.__init__(self, **kwargs)
    self._common_args += ['--ssh_host', WithProperties('%(ssh_host:-None)s'),
                          '--ssh_port', WithProperties('%(ssh_port:-None)s')]

  def Compile(self, clobber=None):
    """Compile step.  Build everything. """
    _args = ['--target', 'all']
    skia_factory.SkiaFactory.Compile(self, clobber)
    # Copy the executables to the device
    self.AddSlaveScript(script='chromeos_send_files.py',
                        description='SendExecutables', halt_on_failure=True)

  def RunTests(self):
    """ Run the unit tests. """
    self.AddSlaveScript(script='chromeos_run_tests.py', description='RunTests')

  def RunGM(self):
    """ Run the "GM" tool, saving the images to disk. """
    self.AddSlaveScript(script='chromeos_run_gm.py', description='GenerateGMs')

  def PreRender(self):
    """ Prepares ChromeOS device for rendering. """
    self.AddSlaveScript(script='chromeos_prerender.py',
                        description='PreRender')

  def RenderPictures(self):
    """ Run the "render_pictures" tool to generate images from .skp's. """
    self.AddSlaveScript(script='chromeos_render_pictures.py',
                        description='RenderPictures')

  def RenderPdfs(self):
    """ Run the "render_pdfs" tool to generate pdfs from .skp's. """
    self.AddSlaveScript(script='chromeos_render_pdfs.py',
                        description='RenderPdfs')

  def PostRender(self):
    """ Post render operations for the ChromeOS device. """
    self.AddSlaveScript(script='chromeos_postrender.py',
                        description='PostRender')

  def RunBench(self):
    """ Run "bench", piping the output somewhere so we can graph
    results over time. """
    self.AddSlaveScript(script='chromeos_run_bench.py', description='RunBench')

  def BenchPictures(self):
    """ Run "bench_pictures" """
    self.AddSlaveScript(script='chromeos_bench_pictures.py',
                        description='BenchPictures')
