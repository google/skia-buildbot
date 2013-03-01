# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Utility class to build the Skia master BuildFactory's.

Based on gclient_factory.py and adds Skia-specific steps."""


from buildbot.process.properties import WithProperties
from config_private import AUTOGEN_SVN_BASEURL, SKIA_SVN_BASEURL
from master.factory import gclient_factory
from master.factory.build_factory import BuildFactory
from skia_master_scripts import commands as skia_commands
import config
import ntpath
import posixpath
import skia_build


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

  def __init__(self, other_subdirs=None, do_upload_results=False,
               do_patch_step=False, build_subdir='trunk',
               target_platform=None, configuration=CONFIG_DEBUG,
               default_timeout=8*60*60,
               environment_variables=None, gm_image_subdir=None,
               perf_output_basedir=None, builder_name=None, flavor=None,
               make_flags=None, test_args=None, gm_args=None, bench_args=None,
               bench_pictures_cfg='default',
               use_skp_playback_framework=False):
    """Instantiates a SkiaFactory as appropriate for this target_platform.

    do_upload_results: whether we should upload bench/gm results
    do_patch_step: whether the build should include a step which applies a
        patch.  This is only applicable for trybots.
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
    flavor: which "flavor" of slave-side scripts this factory should use
    make_flags: list of extra flags to pass to the compile step
    test_args: list of extra flags to pass to the 'tests' executable
    gm_args: list of extra flags to pass to the 'gm' executable
    bench_args: list of extra flags to pass to the 'bench' executable
    bench_pictures_cfg: config name to use for bench_pictures
    use_skp_playback_framework: whether the builder should use the new skp
        playback framework. This is a temporary flag that will be removed once
        all builders use the new framework
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
        svn_url=SKIA_SVN_BASEURL + '/' + build_subdir, name=build_subdir
        ).GetSpec()]

    if not other_subdirs:
      other_subdirs = []
    subdirs_to_checkout = set(other_subdirs)

    # Trybots need to check out all of these directories.
    if do_patch_step:
      subdirs_to_checkout.add('android')
      subdirs_to_checkout.add('gm-expected')
    if not use_skp_playback_framework:
      subdirs_to_checkout.add('skp')
    for other_subdir in subdirs_to_checkout:
      self._gclient_solutions.append(gclient_factory.GClientSolution(
          svn_url=SKIA_SVN_BASEURL + '/' + other_subdir,
          name=other_subdir).GetSpec())

    if gm_image_subdir:
      properties['gm_image_subdir'] = gm_image_subdir

    # Set _default_clobber based on config.Master
    self._default_clobber = getattr(config.Master, 'default_clobber', False)

    self._do_upload_results = do_upload_results
    self._do_upload_bench_results = do_upload_results and \
        perf_output_basedir != None
    self._do_patch_step = do_patch_step

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
    self._flavor = flavor
    self._use_skp_playback_framework = use_skp_playback_framework

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
        '--is_try', str(self._do_patch_step),
        '--bench_pictures_cfg', bench_pictures_cfg,
        '--use_skp_playback_framework', str(self._use_skp_playback_framework),
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

  def AddFlavoredSlaveScript(self, script, **kwargs):
    """ Add a flavor-specific BuildStep.

    Finds a script to run by concatenating the flavor of this BuildFactory with
    the provided script name.
    """
    script_to_run = ('%s_%s' % (self._flavor, script)
                                if self._flavor else script)
    self.AddSlaveScript(script_to_run, **kwargs)

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

    # Trybots should always clean.
    if clobber or self._do_patch_step:
      self.AddSlaveScript(script='clean.py', description='Clean')

    self.Make('skia_base_libs', 'BuildSkiaBaseLibs')
    self.Make('tests', 'BuildTests')
    self.Make('gm', 'BuildGM', is_rebaseline_step=True)
    self.Make('tools', 'BuildTools')
    self.Make('bench', 'BuildBench')
    self.Make('most', 'BuildMost')

  def Install(self):
    """ Install the compiled executables. """
    self.AddFlavoredSlaveScript(script='install.py', description='Install',
                                halt_on_failure=True)

  def DownloadSKPs(self):
    """ Download the SKPs. """
    self.AddSlaveScript(script='download_skps.py', description='DownloadSKPs',
                        halt_on_failure=True)

  def DownloadBaselines(self):
    """ Download the GM baselines. """
    self.AddSlaveScript(script='download_baselines.py',
                        description='DownloadBaselines', halt_on_failure=True)

  def RunTests(self):
    """ Run the unit tests. """
    self.AddFlavoredSlaveScript(script='run_tests.py', description='RunTests')

  def RunGM(self):
    """ Run the "GM" tool, saving the images to disk. """
    self.AddFlavoredSlaveScript(script='run_gm.py', description='GenerateGMs',
                                is_rebaseline_step=True)

  def PreRender(self):
    """ Step to run before the render steps. """
    self.AddFlavoredSlaveScript(script='prerender.py', description='PreRender')

  def RenderPictures(self):
    """ Run the "render_pictures" tool to generate images from .skp's. """
    self.AddFlavoredSlaveScript(script='render_pictures.py',
                                description='RenderPictures')

  def RenderPdfs(self):
    """ Run the "render_pdfs" tool to generate pdfs from .skp's. """
    self.AddFlavoredSlaveScript(script='render_pdfs.py',
                                description='RenderPdfs')

  def PostRender(self):
    """ Step to run after the render steps. """
    self.AddFlavoredSlaveScript(script='postrender.py',
                                description='PostRender')

  def PreBench(self):
    """ Step to run before the benchmarking steps. """
    self.AddFlavoredSlaveScript(script='prebench.py', description='PreBench')

  def PostBench(self):
    """ Step to run after the benchmarking steps. """
    self.AddFlavoredSlaveScript(script='postbench.py', description='PostBench')

  def CompareGMs(self):
    """ Run the "skdiff" tool to compare the "actual" GM images we just
    generated to the baselines in _gm_image_subdir. """
    self.AddSlaveScript(script='compare_gms.py', description='CompareGMs',
                        is_rebaseline_step=True)
  
  def CompareAndUploadWebpageGMs(self):
    """ Run the "skdiff" tool to compare the "actual" GM images we just
    generated to the baselines in _gm_image_subdir and uploads the actual
    images if appropriate. """
    self.AddSlaveScript(script='compare_and_upload_webpage_gms.py',
                        description='CompareAndUploadWebpageGMs',
                        is_rebaseline_step=True)

  def RunBench(self):
    """ Run "bench", piping the output somewhere so we can graph
    results over time. """
    self.AddFlavoredSlaveScript(script='run_bench.py', description='RunBench')

  def BenchPictures(self):
    """ Run "bench_pictures" """
    self.AddFlavoredSlaveScript(script='bench_pictures.py',
                                description='BenchPictures')

  def BenchGraphs(self):
    """ Generate bench performance graphs. """
    self.AddSlaveScript(script='generate_bench_graphs.py',
                        description='GenerateBenchGraphs')

  def GenerateWebpagePictureBenchGraphs(self):
    """ Generate webpage picture bench performance graphs. """
    self.AddSlaveScript(script='generate_webpage_picture_bench_graphs.py',
                        description='GenerateWebpagePictureBenchGraphs')

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

    if self._do_patch_step:
      def _GetPatch(build):
        if build.getSourceStamp().patch:
          patch = (build.getSourceStamp().patch[0],
                   build.getProperty('patch_file_url'))
          return str(patch).encode()
        else:
          patch = 'None'
        return patch

      args = ['--patch', WithProperties('%(patch)s', patch=_GetPatch),
              '--patch_root', WithProperties('%(root:-None)s')]
      self.AddSlaveScript(script='apply_patch.py', description='ApplyPatch',
                          args=args, halt_on_failure=True)

  def UploadBenchGraphs(self):
    """ Upload bench performance graphs (but only if we have been
    recording bench output for this build type). """
    self.AddSlaveScript(script='upload_bench_graphs.py',
                        description='UploadBenchGraphs')

  def UploadWebpagePictureBenchGraphs(self):
    """ Upload webpage picture bench performance graphs (but only if we have
    been recording bench output for this build type). """
    self.AddSlaveScript(script='upload_webpage_picture_bench_graphs.py',
                        description='UploadWebpagePictureBenchGraphs')

  def UploadBenchResults(self):
    """ Upload bench results (performance data). """
    self.AddSlaveScript(script='upload_bench_results.py',
                        description='UploadBenchResults')

  def UploadWebpagePictureBenchResults(self):
    """ Upload webpage picture bench results (performance data). """
    self.AddSlaveScript(script='upload_webpage_picture_bench_results.py',
                        description='UploadWebpagePictureBenchResults')


  def UploadGMResults(self):
    """ Upload the images generated by GM """
    args = ['--autogen_svn_username_file', self._autogen_svn_username_file,
            '--autogen_svn_password_file', self._autogen_svn_password_file]
    self.AddSlaveScript(script='upload_gm_results.py', args=args,
                        description='UploadGMResults', timeout=5400,
                        is_rebaseline_step=True)

  def CommonSteps(self, clobber=None):
    """ Steps which are run at the beginning of all builds. """
    self.UpdateSteps()
    self.DownloadSKPs()
    self.Compile(clobber)
    self.Install()

  def NonPerfSteps(self):
    """ Add correctness testing BuildSteps. """
    self.DownloadBaselines()
    self.PreRender()
    self.RunTests()
    self.RunGM()
    self.RenderPictures()
    self.RenderPdfs()
    self.PostRender()
    if self._do_upload_results:
      self.UploadGMResults()
      if self._use_skp_playback_framework:
        self.CompareAndUploadWebpageGMs()
    self.CompareGMs()

  def PerfSteps(self):
    """ Add performance testing BuildSteps. """
    self.PreBench()
    self.RunBench()
    self.BenchPictures()
    self.PostBench()
    if self._do_upload_bench_results:
      self.UploadBenchResults()
      self.BenchGraphs()
      self.UploadBenchGraphs()
      if self._use_skp_playback_framework:
        self.UploadWebpagePictureBenchResults()
        self.GenerateWebpagePictureBenchGraphs()
        self.UploadWebpagePictureBenchGraphs()

  def Build(self, clobber=None):
    """Build and return the complete BuildFactory.

    clobber: boolean indicating whether we should clean before building
    """
    self.CommonSteps(clobber)
    self.NonPerfSteps()
    self.PerfSteps()
    return self

  def BuildNoPerf(self, clobber=None):
    """Build and return the complete BuildFactory, without the benchmarking
    steps.

    clobber: boolean indicating whether we should clean before building
    """
    self.CommonSteps(clobber)
    self.NonPerfSteps()
    return self

  def BuildPerfOnly(self, clobber=None):
    """Build and return the complete BuildFactory, with only the benchmarking
    steps.

    clobber: boolean indicating whether we should clean before building
    """
    if not self._perf_output_basedir:
      raise ValueError(
          'BuildPerfOnly requires perf_output_basedir to be defined.')
    if self._configuration != CONFIG_RELEASE:
      raise ValueError('BuildPerfOnly should run in %s configuration.' %
                       CONFIG_RELEASE)
    self.CommonSteps(clobber)
    self.PerfSteps()
    return self
