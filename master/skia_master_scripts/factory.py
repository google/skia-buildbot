# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's.

Based on gclient_factory.py and adds Skia-specific steps."""

import ntpath
import posixpath

from buildbot.process.properties import Property, WithProperties
from master.factory import gclient_factory
from master.factory.build_factory import BuildFactory
from skia_master_scripts import commands as skia_commands
import config
import skia_build


SKIA_SVN_BASEURL = 'https://skia.googlecode.com/svn'
AUTOGEN_SVN_BASEURL = 'https://skia-autogen.googlecode.com/svn'

# TODO(epoger): My intent is to make the build steps identical on all platforms
# and thus remove the need for the whole target_platform parameter.
# For now, these must match the target_platform values used in
# third_party/chromium_buildbot/scripts/master/factory/gclient_factory.py ,
# because we pass these values into GClientFactory.__init__() .
TARGET_PLATFORM_LINUX = 'linux'
TARGET_PLATFORM_MAC = 'mac'
TARGET_PLATFORM_WIN32 = 'win32'

CONFIG_DEBUG = 'Debug'
CONFIG_RELEASE = 'Release'
CONFIG_BENCH = 'Bench'
CONFIGURATIONS = [CONFIG_DEBUG, CONFIG_RELEASE]

class SkiaFactory(BuildFactory):
  """Encapsulates data and methods common to the Skia master.cfg files."""

  def __init__(self, do_upload_results=False,
               build_subdir='trunk', other_subdirs=None,
               target_platform=None, configuration=CONFIG_DEBUG,
               default_timeout=8*60*60,
               environment_variables=None, gm_image_subdir=None,
               perf_output_basedir=None, builder_name=None, make_flags=None,
               test_args=None, gm_args=None, bench_args=None,
               bench_pictures_cfg='default'):
    """Instantiates a SkiaFactory as appropriate for this target_platform.

    do_upload_results: whether we should upload bench/gm results
    build_subdir: subdirectory to check out and then build within
    other_subdirs: list of other subdirectories to also check out (or None)
    target_platform: a string such as TARGET_PLATFORM_LINUX
    configuration: 'Debug' or 'Release'
    default_timeout: default timeout for each command, in seconds
    environment_variables: dictionary of environment variables that should
        be passed to all commands
    gm_image_subdir: directory containing images for comparison against results
        of gm tool
    perf_output_basedir: path to directory under which to store performance
        data, or None if we don't want to store performance data
    builder_name: name of the builder associated with this factory
    make_flags: list of extra flags to pass to the compile step
    test_args: list of extra flags to pass to the 'tests' executable
    gm_args: list of extra flags to pass to the 'gm' executable
    bench_args: list of extra flags to pass to the 'bench' executable
    bench_pictures_cfg: config name to use for bench_pictures
    """
    properties = {}

    if not make_flags:
      make_flags = []
    self._make_flags = make_flags
    # Platform-specific stuff.
    if target_platform == TARGET_PLATFORM_WIN32:
      self.TargetPathJoin = ntpath.join
    else:
      self.TargetPathJoin = posixpath.join
      self._make_flags += ['--jobs', '--max-load=4.0']

    # Create gclient solutions corresponding to the main build_subdir
    # and other directories we also wish to check out.
    self._gclient_solutions = [gclient_factory.GClientSolution(
        svn_url=config.Master.skia_url + build_subdir, name=build_subdir
        ).GetSpec()]
    if not other_subdirs:
      other_subdirs = []
    if gm_image_subdir:
      other_subdirs.append('gm-expected/%s' % gm_image_subdir)
    other_subdirs.append('skp')
    for other_subdir in other_subdirs:
      self._gclient_solutions.append(gclient_factory.GClientSolution(
          svn_url=config.Master.skia_url + other_subdir,
          name=other_subdir).GetSpec())

    if gm_image_subdir:
      properties['gm_image_subdir'] = gm_image_subdir

    # Set _default_clobber based on config.Master
    self._default_clobber = getattr(config.Master, 'default_clobber', False)

    self._do_upload_results = do_upload_results
    self._do_upload_bench_results = do_upload_results and \
        perf_output_basedir != None

    # Get an implementation of SkiaCommands as appropriate for
    # this target_platform.
    workdir = self.TargetPathJoin('build', build_subdir)
    self._skia_cmd_obj = skia_commands.SkiaCommands(
        target_platform=target_platform, factory=self,
        configuration=configuration, workdir=workdir,
        target_arch=None, default_timeout=default_timeout,
        environment_variables=environment_variables)

    self._perf_output_basedir = perf_output_basedir

    self._configuration = configuration
    if self._configuration not in CONFIGURATIONS:
      raise ValueError('Invalid configuration %s.  Must be one of: %s' % (
          self._configuration, CONFIGURATIONS))

    self._skia_svn_username_file = '.skia_svn_username'
    self._skia_svn_password_file = '.skia_svn_password'
    self._autogen_svn_username_file = '.autogen_svn_username'
    self._autogen_svn_password_file = '.autogen_svn_password'
    self._builder_name = builder_name

    # The class to use when creating builds in build_factory.BuildFactory
    self.buildClass = skia_build.SkiaBuild

    def _DetermineRevision(build):
      """ Get the 'revision' property at build time. WithProperties returns the
      empty string if 'revision' is not defined, which causes failures when we
      try to pass the revision over a command line, so we use the string "None"
      to indicate that the revision is not defined.

      build: instance of Build for the current build.
      """
      props = build.getProperties().asDict()
      if props.has_key('revision'):
        if props['revision'][0]:
          return props['revision'][0]
      return 'None'

    if not test_args:
      test_args = []
    if not gm_args:
      gm_args = []
    if not bench_args:
      bench_args = []

    self._common_args = [
        '--autogen_svn_baseurl', AUTOGEN_SVN_BASEURL,
        '--configuration', configuration,
        '--do_upload_results', str(self._do_upload_results),
        '--gm_image_subdir', gm_image_subdir or 'None',
        '--builder_name', builder_name,
        '--target_platform', target_platform,
        '--revision', WithProperties('%(rev)s', rev=_DetermineRevision),
        '--got_revision', WithProperties('%(got_revision:-None)s'),
        '--perf_output_basedir', perf_output_basedir or 'None',
        '--make_flags', '"%s"' % ' '.join(self._make_flags),
        '--test_args', '"%s"' % ' '.join(test_args),
        '--gm_args', '"%s"' % ' '.join(gm_args),
        '--bench_args', '"%s"' % ' '.join(bench_args),
        '--num_cores', WithProperties('%(num_cores:-None)s'),
        '--bench_pictures_cfg', bench_pictures_cfg,
        ]
    BuildFactory.__init__(self, build_factory_properties=properties)

  def AddSlaveScript(self, script, description, args=None, timeout=None,
                     halt_on_failure=False, is_upload_step=False,
                     is_rebaseline_step=False, get_props_from_stdout=None,
                     workdir=None):
    """ Add a BuildStep consisting of a python script.

    script: which slave-side python script to run.
    description: string briefly describing the BuildStep.
    args: optional list of strings; arguments to pass to the script.
    timeout: optional integer; maximum time for the BuildStep to complete.
    halt_on_failure: boolean indicating whether to continue the build if this
        step fails.
    is_upload_step: boolean indicating whether this step should be skipped when
        the buildbot is not performing uploads.
    is_rebaseline_step: boolean indicating whether this step is required for
        rebaseline-only builds.
    get_props_from_stdout: optional dictionary. Keys are strings indicating
        build properties to set based on the output of this step. Values are
        strings containing regular expressions for parsing the property from
        the output of the step.
    workdir: optional string indicating the working directory in which to run
        the script. If this is provided, then the script must be given relative
        to this directory.
    """
    arguments = self._common_args
    if args:
      arguments += args
    self._skia_cmd_obj.AddSlaveScript(
        script=script,
        args=arguments,
        description=description,
        timeout=timeout,
        halt_on_failure=halt_on_failure,
        is_upload_step=is_upload_step,
        is_rebaseline_step=is_rebaseline_step,
        get_props_from_stdout=get_props_from_stdout,
        workdir=workdir)

  def Make(self, target, description, is_rebaseline_step=False):
    """ Build a single target."""
    args = ['--target', target]
    self.AddSlaveScript(script='compile.py', args=args,
                        description=description, halt_on_failure=True,
                        is_rebaseline_step=is_rebaseline_step)

  def Compile(self, clobber=None):
    """ Compile step. Build everything.

    clobber: optional boolean which tells us whether to 'clean' before building.
    """
    if clobber is None:
      clobber = self._default_clobber
    if clobber:
      self.AddSlaveScript(script='clean.py', description='Clean')
    self.Make('skia_base_libs',  'BuildSkiaBaseLibs')
    self.Make('tests', 'BuildTests')
    self.Make('gm',    'BuildGM', is_rebaseline_step=True)
    self.Make('tools', 'BuildTools')
    self.Make('bench', 'BuildBench')
    self.Make('most',  'BuildMost')

  def RunTests(self):
    """ Run the unit tests. """
    self.AddSlaveScript(script='run_tests.py', description='RunTests')

  def RunGM(self):
    """ Run the "GM" tool, saving the images to disk. """
    self.AddSlaveScript(script='run_gm.py', description='GenerateGMs',
                        is_rebaseline_step=True)

  def RenderPictures(self):
    """ Run the "render_pictures" tool to generate images from .skp's. """
    self.AddSlaveScript(script='render_pictures.py',
                        description='RenderPictures')

  def CompareGMs(self):
    """ Run the "skdiff" tool to compare the "actual" GM images we just
    generated to the baselines in _gm_image_subdir. """
    self.AddSlaveScript(script='compare_gms.py', description='CompareGMs',
                        is_rebaseline_step=True)

  def RunBench(self):
    """ Run "bench", piping the output somewhere so we can graph
    results over time. """
    self.AddSlaveScript(script='run_bench.py', description='RunBench')

  def BenchPictures(self):
    """ Run "bench_pictures" """
    self.AddSlaveScript(script='bench_pictures.py', description='BenchPictures')

  def BenchGraphs(self):
    """ Generate bench performance graphs. """
    self.AddSlaveScript(script='generate_bench_graphs.py',
                        description='GenerateBenchGraphs')

  def UpdateSteps(self):
    """ Update the Skia sources. """
    self.AddSlaveScript(script=self.TargetPathJoin('..', '..', '..', '..', '..',
                                                   'slave',
                                                   'skia_slave_scripts',
                                                   'update_scripts.py'),
                        description='UpdateScripts',
                        halt_on_failure=True,
                        workdir='build')
    args = ['--gclient_solutions', '"%s"' % self._gclient_solutions]
    self.AddSlaveScript(
        script=self.TargetPathJoin('..', '..', '..', '..', '..', 'slave',
                                   'skia_slave_scripts', 'update.py'),
        description='Update',
        args=args,
        timeout=None,
        halt_on_failure=True,
        is_upload_step=False,
        is_rebaseline_step=True,
        get_props_from_stdout={'got_revision':'Skia updated to revision (\d+)'},
        workdir='build')

  def UploadBenchGraphs(self):
    """ Upload bench performance graphs (but only if we have been
    recording bench output for this build type). """
    self.AddSlaveScript(script='upload_bench_graphs.py',
                        description='UploadBenchGraphs')

  def UploadBenchResults(self):
    """ Upload bench results (performance data). """
    self.AddSlaveScript(script='upload_bench_results.py',
                        description='UploadBenchResults')

  def UploadGMResults(self):
    """ Upload the images generated by GM """
    args = ['--autogen_svn_username_file', self._autogen_svn_username_file,
            '--autogen_svn_password_file', self._autogen_svn_password_file]
    self.AddSlaveScript(script='upload_gm_results.py', args=args,
                        description='UploadGMResults', timeout=5400,
                        is_rebaseline_step=True)

  def NonPerfSteps(self):
    """ Add correctness testing BuildSteps. """
    self.RunTests()
    self.RunGM()
    self.RenderPictures()
    if self._do_upload_results:
      self.UploadGMResults()
    self.CompareGMs()

  def PerfSteps(self):
    """ Add performance testing BuildSteps. """
    self.RunBench()
    self.BenchPictures()
    if self._do_upload_bench_results:
      self.UploadBenchResults()
      self.BenchGraphs()
      self.UploadBenchGraphs()

  def Build(self, clobber=None):
    """Build and return the complete BuildFactory.

    clobber: boolean indicating whether we should clean before building
    """
    self.UpdateSteps()
    self.Compile(clobber)
    self.NonPerfSteps()
    self.PerfSteps()

    return self
