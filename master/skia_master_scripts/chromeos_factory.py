# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for ChromeOS buildbots.

Overrides SkiaFactory with any ChromeOS-specific steps."""


from skia_master_scripts import factory as skia_factory


class ChromeOSFactory(skia_factory.SkiaFactory):
  """Overrides for ChromeOS builds."""

  def __init__(self, ssh_host, ssh_port, do_upload_results=False,
               build_subdir='trunk', other_subdirs=None, target_platform=None,
               configuration=skia_factory.CONFIG_DEBUG, default_timeout=600,
               environment_variables=None, gm_image_subdir=None,
               perf_output_basedir=None, builder_name=None, make_flags=None,
               test_args=None, gm_args=None, bench_args=None):
    """ Instantiates a ChromeOSFactory with properties and build steps specific
    to ChromeOS devices.

    ssh_host: string indicating hostname or ip address of the target device
    ssh_port: string indicating the ssh port on the target device
    """
    super(ChromeOSFactory, self).__init__(
        do_upload_results=do_upload_results,
        build_subdir=build_subdir,
        other_subdirs=other_subdirs,
        target_platform=target_platform,
        configuration=configuration,
        default_timeout=default_timeout,
        environment_variables=environment_variables,
        gm_image_subdir=gm_image_subdir,
        perf_output_basedir=perf_output_basedir,
        builder_name=builder_name,
        make_flags=make_flags,
        test_args=test_args,
        gm_args=gm_args,
        bench_args=bench_args)
    self._common_args += ['--ssh_host', ssh_host,
                          '--ssh_port', ssh_port]

  def Compile(self, clobber=None):
    """Compile step.  Build everything. """
    args = ['--target', 'all']
    super(ChromeOSFactory, self).Compile(clobber)
    # Copy the executables to the device
    self.AddSlaveScript(script='chromeos_send_files.py',
                        description='SendExecutables', halt_on_failure=True)


  def RunTests(self):
    """ Run the unit tests. """
    self.AddSlaveScript(script='chromeos_run_tests.py', description='RunTests')


  def RunGM(self):
    """ Run the "GM" tool, saving the images to disk. """
    self.AddSlaveScript(script='chromeos_run_gm.py', description='GenerateGMs')


  def RenderPictures(self):
    """ Run the "render_pictures" tool to generate images from .skp's. """
    self.AddSlaveScript(script='chromeos_render_pictures.py',
                        description='RenderPictures')


  def RunBench(self):
    """ Run "bench", piping the output somewhere so we can graph
    results over time. """
    self.AddSlaveScript(script='chromeos_run_bench.py', description='RunBench')


  def BenchPictures(self):
    """ Run "bench_pictures" """
    self.AddSlaveScript(script='chromeos_bench_pictures.py',
                        description='BenchPictures')