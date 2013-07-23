# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Utility class to build the Skia master BuildFactory's.

Based on gclient_factory.py and adds Skia-specific steps."""


from buildbot.process.properties import WithProperties
from buildbot.status import builder
from config_private import AUTOGEN_SVN_BASEURL, SKIA_SVN_BASEURL
from master.factory import gclient_factory
from master.factory.build_factory import BuildFactory
from skia_master_scripts import commands as skia_commands

import builder_name_schema
import config
import config_private
import ntpath
import os
import posixpath
import utils


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
CONFIGURATIONS = [CONFIG_DEBUG, CONFIG_RELEASE]


_COMPILE_STEP_PREFIX = 'Build'
_COMPILE_RETRY_PREFIX = 'Retry_' + _COMPILE_STEP_PREFIX
_COMPILE_NO_WERR_PREFIX = 'Retry_NoWarningsAsErrors_' + _COMPILE_STEP_PREFIX


class SkiaFactory(BuildFactory):
  """Encapsulates data and methods common to the Skia master.cfg files."""

  def __init__(self, other_subdirs=None, do_upload_results=False,
               do_patch_step=False, build_subdir='trunk',
               target_platform=None, configuration=CONFIG_DEBUG,
               default_timeout=8*60*60, deps_target_os=None,
               environment_variables=None, gm_image_subdir=None,
               perf_output_basedir=None, builder_name=None, flavor=None,
               make_flags=None, test_args=None, gm_args=None, bench_args=None,
               bench_pictures_cfg='default', compile_warnings_as_errors=False,
               gyp_defines=None, build_targets=None):
    """Instantiates a SkiaFactory as appropriate for this target_platform.

    do_upload_results: whether we should upload bench/gm results
    do_patch_step: whether the build should include a step which applies a
        patch.  This is only applicable for trybots.
    build_subdir: subdirectory to check out and then build within
    other_subdirs: list of other subdirectories to also check out (or None)
    target_platform: a string such as TARGET_PLATFORM_LINUX
    configuration: 'Debug' or 'Release'
    default_timeout: default timeout for each command, in seconds
    deps_target_os: string; the target_os to be specified in the gclient config.
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
    compile_warnings_as_errors: boolean; whether to build with "-Werror" or
        some equivalent.
    gyp_defines: optional dict; GYP_DEFINES to be used in the build.
    build_targets: optional list; the targets to build. Default is set depending
        on which Build() function is called.
    """
    properties = {}

    if not make_flags:
      make_flags = []
    self._make_flags = make_flags
    # Platform-specific stuff.
    if target_platform == TARGET_PLATFORM_WIN32:
      self.TargetPath = ntpath
    else:
      self.TargetPath = posixpath

    # Create gclient solutions corresponding to the main build_subdir
    # and other directories we also wish to check out.
    self._gclient_solutions = [gclient_factory.GClientSolution(
        svn_url=SKIA_SVN_BASEURL + '/' + build_subdir, name=build_subdir
        ).GetSpec()]

    if not other_subdirs:
      other_subdirs = []
    subdirs_to_checkout = set(other_subdirs)

    # Trybots need to check out all of these directories.
    if do_patch_step and not \
        builder_name.startswith(builder_name_schema.BUILDER_ROLE_BUILD):
      subdirs_to_checkout.add('gm-expected')
    for other_subdir in subdirs_to_checkout:
      self._gclient_solutions.append(gclient_factory.GClientSolution(
          svn_url=SKIA_SVN_BASEURL + '/' + other_subdir,
          name=other_subdir).GetSpec())

    self._deps_target_os = deps_target_os

    if gm_image_subdir:
      properties['gm_image_subdir'] = gm_image_subdir

    # Set _default_clobber based on config.Master
    self._default_clobber = getattr(config.Master, 'default_clobber', False)

    self._do_upload_results = do_upload_results
    self._do_upload_bench_results = do_upload_results and \
        perf_output_basedir != None
    self._do_patch_step = do_patch_step

    if not environment_variables:
      self._env_vars = {}
    else:
      self._env_vars = dict(environment_variables)

    self._gyp_defines = dict(gyp_defines or {})
    self._gyp_defines['skia_warnings_as_errors'] = \
        '%d' % int(compile_warnings_as_errors)

    self._build_targets = list(build_targets or [])

    # Get an implementation of SkiaCommands as appropriate for
    # this target_platform.
    self._workdir = self.TargetPath.join('build', build_subdir)
    self._skia_cmd_obj = skia_commands.SkiaCommands(
        target_platform=target_platform, factory=self,
        configuration=configuration, workdir=self._workdir,
        target_arch=None, default_timeout=default_timeout,
        environment_variables=self._env_vars)

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
        '--deps_target_os', self._deps_target_os or 'None',
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
        '--flavor', self._flavor or 'default',
        ]
    BuildFactory.__init__(self, build_factory_properties=properties)

  def Validate(self):
    """ Validate the Factory against the known good configuration. """
    test_dir = os.path.join(os.path.dirname(__file__), os.pardir, os.pardir,
                            'tools', 'tests', 'factory_configuration')

    # Write the actual configuration.
    actual_dir = os.path.join(test_dir, 'actual')
    if not os.path.exists(actual_dir):
      os.makedirs(actual_dir)
    self_as_string = utils.ToString(self.__dict__)
    with open(os.path.join(actual_dir, self._builder_name), 'w') as f:
      f.write(self_as_string)

    # Read the expected configuration.
    expected_dir = os.path.join(test_dir, 'expected')
    try:
      expectation = open(os.path.join(expected_dir, self._builder_name)).read()
    except IOError:
      msg = 'No expected factory configuration for %s.' % self._builder_name
      if config_private.die_on_validation_failure:
        raise Exception(msg)
      else:
        print 'Warning: %s' % msg
        return

    # Compare actual to expected.
    if self_as_string != expectation:
      if config_private.die_on_validation_failure:
        raise ValueError('Factory configuration for %s does not match '
                         'expectation!  Here\'s the diff:\n%s\n' %
                         (self._builder_name, utils.StringDiff(
                             self_as_string, expectation)))
      else:
        # We don't print the full diff in this case because:
        # a. It's generally too long to be easily read in a terminal
        # b. All of the printing can noticeably slow down the master startup
        # c. The master prints so much output that it would be easy to miss the
        #    diff if we did print it.
        print 'Warning: Factory configuration for %s does not match ' \
              'expectation!' % self._builder_name

  def AddSlaveScript(self, script, description, args=None, timeout=None,
                     halt_on_failure=False, is_upload_step=False,
                     is_rebaseline_step=False, get_props_from_stdout=None,
                     workdir=None, do_step_if=None, always_run=False,
                     flunk_on_failure=True):
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
    do_step_if: optional, function which determines whether or not to run the
        step.
    always_run: boolean indicating whether this step should run even if a
        previous step which had halt_on_failure has failed.
    flunk_on_failure: boolean indicating whether the whole build fails if this
        step fails.
    """
    arguments = list(self._common_args)
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
        workdir=workdir,
        do_step_if=do_step_if,
        always_run=always_run,
        flunk_on_failure=flunk_on_failure)

  def AddFlavoredSlaveScript(self, script, **kwargs):
    """ Add a flavor-specific BuildStep.

    Finds a script to run by concatenating the flavor of this BuildFactory with
    the provided script name.
    """
    script_to_run = ('%s_%s' % (self._flavor, script)
                                if self._flavor else script)
    self.AddSlaveScript(script_to_run, **kwargs)

  def RunGYP(self):
    """ Run GYP to generate build files. """
    self.AddSlaveScript(script='run_gyp.py', description='RunGYP',
                        halt_on_failure=True,
                        args=['--gyp_defines',
                                  ' '.join('%s=%s' % (k, v) for k, v in
                                           self._gyp_defines.items())])

  def Make(self, target, description, is_rebaseline_step=False, do_step_if=None,
           always_run=False, flunk_on_failure=True, halt_on_failure=True):
    """ Build a single target.

    target: string; the target to build.
    description: string; description of this BuildStep.
    is_rebaseline_step: optional boolean; whether or not this step is required
        for rebaseline-only builds.
    do_step_if: optional, function which determines whether or not to run this
        step.
    always_run: boolean indicating whether this step should run even if a
        previous step which had halt_on_failure has failed.
    flunk_on_failure: boolean indicating whether the whole build fails if this
        step fails.
    halt_on_failure: boolean indicating whether to continue the build if this
        step fails.
    """
    args = ['--target', target,
            '--gyp_defines',
            ' '.join('%s=%s' % (k, v) for k, v in self._gyp_defines.items())]
    self.AddFlavoredSlaveScript(script='compile.py', args=args,
                                description=description,
                                halt_on_failure=halt_on_failure,
                                is_rebaseline_step=is_rebaseline_step,
                                do_step_if=do_step_if,
                                always_run=always_run,
                                flunk_on_failure=flunk_on_failure)

  def Compile(self, clobber=None, retry_with_clobber_on_failure=True,
              retry_without_werr_on_failure=False):
    """ Compile step. Build everything.

    clobber: optional boolean; whether to 'clean' before building.
    retry_with_clobber_on_failure: optional boolean; if the build fails, clean
        and try again, with the same configuration as before.
    retry_without_werr_on_failure: optional boolean; if the build fails, clean
        and try again *without* warnings-as-errors.
    """
    if clobber is None:
      clobber = self._default_clobber

    # Trybots should always clean.
    if clobber or self._do_patch_step:
      self.AddSlaveScript(script='clean.py', description='Clean',
                          halt_on_failure=True)

    # Always re-run gyp before compiling.
    self.RunGYP()

    # Only retry with clobber if we've requested it AND we aren't clobbering on
    # the first build.
    maybe_retry_with_clobber = retry_with_clobber_on_failure and not clobber

    def ShouldRetryWithClobber(step):
      """ Determine whether the retry step should run. """
      if not maybe_retry_with_clobber:
        return False
      compile_failed = False
      retry_failed = False
      for build_step in step.build.getStatus().getSteps():
        if (build_step.isFinished() and
            build_step.getResults()[0] == builder.FAILURE):
          if build_step.getName().startswith(_COMPILE_STEP_PREFIX):
            compile_failed = True
          elif build_step.getName().startswith(_COMPILE_RETRY_PREFIX):
            retry_failed = True
      return compile_failed and not retry_failed

    def ShouldRetryWithoutWarnings(step):
      """ Determine whether the retry-without-warnings-as-errors step should
      run. """
      if not retry_without_werr_on_failure:
        return False
      compile_failed = False
      retry_failed = False
      no_warning_retry_failed = False
      for build_step in step.build.getStatus().getSteps():
        if (build_step.isFinished() and
            build_step.getResults()[0] == builder.FAILURE):
          if build_step.getName().startswith(_COMPILE_STEP_PREFIX):
            compile_failed = True
          elif build_step.getName().startswith(_COMPILE_RETRY_PREFIX):
            retry_failed = True
          elif build_step.getName().startswith(
              _COMPILE_NO_WERR_PREFIX):
            no_warning_retry_failed = True
      # If we've already failed a previous retry without warnings, just give up.
      if no_warning_retry_failed:
        return False
      # If we're retrying with clobber, only retry without warnings if a clobber
      # retry has failed.
      if maybe_retry_with_clobber:
        return retry_failed
      # Only run the retry if the initial compile has failed.
      return compile_failed

    for build_target in self._build_targets:
      self.Make(target=build_target,
                description=_COMPILE_STEP_PREFIX + \
                    utils.UnderscoresToCapWords(build_target),
                flunk_on_failure=not maybe_retry_with_clobber,
                halt_on_failure=(not maybe_retry_with_clobber and
                                 not retry_without_werr_on_failure))

    # Try again with a clean build.
    self.AddSlaveScript(script='clean.py', description='Clean',
                        do_step_if=ShouldRetryWithClobber)
    for build_target in self._build_targets:
      self.Make(target=build_target,
                description=_COMPILE_RETRY_PREFIX + \
                    utils.UnderscoresToCapWords(build_target),
                flunk_on_failure=True,
                halt_on_failure=not retry_without_werr_on_failure,
                do_step_if=ShouldRetryWithClobber)

    # Try again without warnings-as-errors.
    self._gyp_defines['skia_warnings_as_errors'] = '0'
    self.AddSlaveScript(script='clean.py', description='Clean',
                        always_run=True,
                        do_step_if=ShouldRetryWithoutWarnings)
    for build_target in self._build_targets:
      self.Make(target=build_target,
                description=_COMPILE_NO_WERR_PREFIX + \
                    utils.UnderscoresToCapWords(build_target),
                flunk_on_failure=True,
                halt_on_failure=True,
                do_step_if=ShouldRetryWithoutWarnings)

  def Install(self):
    """ Install the compiled executables. """
    self.AddFlavoredSlaveScript(script='install.py', description='Install',
                                halt_on_failure=True)

  def DownloadSKPs(self):
    """ Download the SKPs. """
    self.AddSlaveScript(script='download_skps.py', description='DownloadSKPs',
                        halt_on_failure=True)

  def DownloadSKImageFiles(self):
    """ Download image files for running skimage. """
    self.AddSlaveScript(script='download_skimage_files.py',
                        description='DownloadSKImageFiles',
                        halt_on_failure=True)

  def DownloadBaselines(self):
    """ Download the GM baselines. """
    self.AddSlaveScript(script='download_baselines.py',
                        description='DownloadBaselines', halt_on_failure=True)

  def RunTests(self):
    """ Run the unit tests. """
    self.AddFlavoredSlaveScript(script='run_tests.py', description='RunTests')

  def RunDecodingTests(self):
    """ Run tests of image decoders. """
    self.AddFlavoredSlaveScript(script='run_decoding_tests.py',
                                description='RunDecodingTests')

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

  def CheckForRegressions(self):
    """ Check for benchmark regressions. """
    self.AddSlaveScript(script='check_for_regressions.py',
                        description='CheckForRegressions')

  def BenchGraphs(self):
    """ Generate bench performance graphs. """
    self.AddSlaveScript(script='generate_bench_graphs.py',
                        description='GenerateBenchGraphs')

  def GenerateWebpagePictureBenchGraphs(self):
    """ Generate webpage picture bench performance graphs. """
    self.AddSlaveScript(script='generate_webpage_picture_bench_graphs.py',
                        description='GenerateWebpagePictureBenchGraphs')

  def UpdateScripts(self):
    """ Update the buildbot scripts on the build slave. """
    self.AddSlaveScript(script=self.TargetPath.join('..', '..', '..', '..',
                                                    '..', 'slave',
                                                    'skia_slave_scripts',
                                                    'update_scripts.py'),
                        description='UpdateScripts',
                        halt_on_failure=True,
                        workdir='build')

  def Update(self):
    """ Update the Skia code on the build slave. """
    args = ['--gclient_solutions', '"%s"' % self._gclient_solutions]
    self.AddSlaveScript(
        script=self.TargetPath.join('..', '..', '..', '..', '..', 'slave',
                                   'skia_slave_scripts', 'update.py'),
        description='Update',
        args=args,
        timeout=None,
        halt_on_failure=True,
        is_upload_step=False,
        is_rebaseline_step=True,
        get_props_from_stdout={'got_revision':'Skia updated to revision (\d+)'},
        workdir='build')

  def ApplyPatch(self, alternate_workdir=None, alternate_script=None):
    """ Apply a patch to the Skia code on the build slave. """
    def _GetRoot(build):
      if build.getSourceStamp().patch and 'root' in build.getProperties():
        return build.getProperty('root')
      elif 'issue' in build.getProperties() and \
           'patchset' in build.getProperties() and \
           'baseurl' in build.getProperties():
        baseurl = build.getProperty('baseurl')
        if len(baseurl) == 1:
          baseurl = baseurl[0]
        baseurl = baseurl.split('://', 1)[1]
        skia_baseurl = config_private.SKIA_SVN_BASEURL.split('://', 1)[1]
        if baseurl.startswith(skia_baseurl):
          return baseurl.split(skia_baseurl)[1].strip('/')
      return 'None'

    def _GetPatch(build):
      if build.getSourceStamp().patch and \
          'patch_file_url' in build.getProperties():
        patch = (build.getSourceStamp().patch[0],
                 build.getProperty('patch_file_url'))
        return str(patch).encode()
      elif 'issue' in build.getProperties() and \
          'patchset' in build.getProperties():
        patch = '%s/download/issue%d_%d.diff' % (
            config_private.CODE_REVIEW_SITE.rstrip('/'),
            build.getProperty('issue'),
            build.getProperty('patchset'))
        return str((0, patch)).encode()
      else:
        patch = 'None'
      return patch

    if not bool(alternate_workdir) == bool(alternate_script):
      raise ValueError('alternate_workdir and alternate_script must be provided'
                       ' together.')
    args = ['--patch', WithProperties('%(patch)s', patch=_GetPatch),
            '--patch_root', WithProperties('%(root)s', root=_GetRoot)]
    if alternate_script:
      self.AddSlaveScript(script=alternate_script, description='ApplyPatch',
                          args=args, halt_on_failure=True,
                          workdir=alternate_workdir)
    else:
      self.AddSlaveScript(script='apply_patch.py', description='ApplyPatch',
                          args=args, halt_on_failure=True)

  def UpdateSteps(self):
    """ Update the Skia sources. """
    self.UpdateScripts()
    self.Update()
    if self._do_patch_step:
      self.ApplyPatch()

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

  def UploadBenchResultsToAppEngine(self):
    """ Upload bench results (performance data) to AppEngine. """
    self.AddSlaveScript(script='upload_bench_results_appengine.py',
                        description='UploadBenchResultsToAppengine')

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

  def UploadSKImageResults(self):
    self.AddSlaveScript(script='upload_skimage_results.py',
                        description="UploadSKImageResults")

  def CommonSteps(self, clobber=None):
    """ Steps which are run at the beginning of all builds. """
    self.UpdateSteps()
    self.DownloadSKPs()
    self.Compile(clobber)
    self.Install()

  def NonPerfSteps(self):
    """ Add correctness testing BuildSteps. """
    self.DownloadBaselines()
    self.DownloadSKImageFiles()
    self.PreRender()
    self.RunTests()
    self.RunGM()
    self.RenderPictures()
    self.RenderPdfs()
    self.RunDecodingTests()
    self.PostRender()
    if self._do_upload_results:
      self.UploadGMResults()
      self.CompareAndUploadWebpageGMs()
      self.UploadSKImageResults()
    self.CompareGMs()

  def PerfSteps(self):
    """ Add performance testing BuildSteps. """
    self.PreBench()
    self.RunBench()
    self.BenchPictures()
    self.PostBench()
    self.CheckForRegressions()
    self.BenchGraphs()
    if self._do_upload_bench_results:
      self.UploadBenchResults()
      self.UploadBenchResultsToAppEngine()
      self.UploadBenchGraphs()

  def Build(self, role=None, clobber=None):
    """Build and return the complete BuildFactory.

    role: string; the intended role of this builder. The role affects which
        steps are run. Known values are given in the utils module.
    clobber: boolean indicating whether we should clean before building
    """
    if not role:
      # If no role is provided, just run everything.
      if not self._build_targets:
        self._build_targets = ['most']
      self.CommonSteps(clobber)
      self.NonPerfSteps()
      self.PerfSteps()
    elif role == builder_name_schema.BUILDER_ROLE_BUILD:
      # Compile-only builder.
      self.UpdateSteps()
      if not self._build_targets:
        self._build_targets = ['skia_lib', 'tests', 'gm', 'tools', 'bench',
                               'most']
      self.Compile(clobber=clobber,
                   retry_without_werr_on_failure=True)
    else:
      if not self._build_targets:
        self._build_targets = ['most']
      self.CommonSteps(clobber)
      if role == builder_name_schema.BUILDER_ROLE_TEST:
        # Test-running builder.
        self.NonPerfSteps()
        if self._configuration == CONFIG_DEBUG:
          # Debug-mode testers run all steps, but release-mode testers don't.
          self.PerfSteps()
      elif role == builder_name_schema.BUILDER_ROLE_PERF:
        # Perf-only builder.
        if not self._perf_output_basedir:
          raise ValueError(
              'BuildPerfOnly requires perf_output_basedir to be defined.')
        if self._configuration != CONFIG_RELEASE:
          raise ValueError('BuildPerfOnly should run in %s configuration.' %
                           CONFIG_RELEASE)
        self.PerfSteps()
    self.Validate()
    return self
