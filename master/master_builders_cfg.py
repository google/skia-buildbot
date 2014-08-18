# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.


#pylint: disable=C0301


from skia_master_scripts.canary_factory import CanaryFactory as f_canary
from skia_master_scripts.chromeos_factory import ChromeOSFactory as f_cros
from skia_master_scripts.drt_canary_factory import DRTCanaryFactory as f_drt
from skia_master_scripts.factory import SkiaFactory as f_factory
from skia_master_scripts import utils
from skia_master_scripts.xsan_factory import XsanFactory as f_xsan
from skia_master_scripts.factory import TARGET_PLATFORM_LINUX as LINUX
from skia_master_scripts.factory import TARGET_PLATFORM_WIN32 as WIN32
from skia_master_scripts.factory import TARGET_PLATFORM_MAC as MAC

import builder_name_schema
import collections


# Directory where we want to record performance data
#
# TODO(epoger): consider changing to reuse existing config.Master.perf_base_url,
# config.Master.perf_report_url_suffix, etc.
perf_output_basedir_linux = '../../../../perfdata'
perf_output_basedir_mac = perf_output_basedir_linux
perf_output_basedir_windows = '..\\..\\..\\..\\perfdata'


# Maps a target platform to a perf_output_basedir.
PERF_OUTPUT_BASEDIR = {
  LINUX: perf_output_basedir_linux,
  MAC: perf_output_basedir_mac,
  WIN32: perf_output_basedir_windows,
}


# Schedulers used by the build master.
S_PERCOMMIT = 'skia_percommit_scheduler'
S_NIGHTLY = 'skia_nightly_scheduler'
S_15MINS = 'skia_15_minute_scheduler'


defaults = {}


ARCH_TO_GYP_DEFINE = {
  'x86': {'skia_arch_width': '32'},
  'x86_64': {'skia_arch_width': '64'},
  'Arm7': {'skia_arch_width': '32'},
  'Arm64': {'skia_arch_width': '64'},
  'Mips': {'skia_arch_width': '32'},
  'Mips64': {'skia_arch_width': '64'},
  'MipsDSP2': {'skia_arch_width': '32'},
  'NaCl': None,
}


CHROMEOS_BOARD_NAME = {
  'Alex': 'x86-alex',
  'Link': 'link',
  'Daisy': 'daisy',
}


# GYP_DEFINES for various types of builders.
GYP_WIN7 = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                 'qt_sdk': 'C:/Qt/4.8.5/'})
GYP_WIN8 = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                 'qt_sdk': 'C:/Qt/Qt5.1.0/5.1.0/msvc2012_64/'})
GYP_ANGLE = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                  'qt_sdk': 'C:/Qt/4.8.5/',
                  'skia_angle': '1'})
GYP_GDI = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                'qt_sdk': 'C:/Qt/4.8.5/',
                'skia_gdi': '1'})
GYP_EXC = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                'qt_sdk': 'C:/Qt/4.8.5/',
                'skia_win_exceptions': '1'})
GYP_IOS = repr({'skia_os': 'ios'})
NO_GPU = repr({'skia_gpu': '0'})
CLANG = repr({'skia_clang_build': '1'})
VALGRIND = repr({'skia_release_optimization_level': '1'})
PDFVIEWER = repr({'skia_run_pdfviewer_in_gm': '1'})
GYP_SHARED = repr({'component': 'shared_library'})
NO_GPU_SHARED = repr({'skia_gpu': '0', 'skia_shared_lib': '1'})


def get_gyp_defines(arch, gyp_defines_str=None):
  """Build a dictionary of gyp defines.

  Args:
      arch: string; target architecture. Needs to appear in ARCH_TO_GYP_DEFINE.
      gyp_defines_str: optional string; a string representation of a dictionary
          of gyp defines.
  Returns:
      a dictionary mapping gyp variable names to their intended values.
  """
  try:
    arch_width_define = ARCH_TO_GYP_DEFINE[arch]
  except KeyError:
    raise Exception('Unknown arch type: %s' % arch)
  gyp_defines = eval(gyp_defines_str or 'None')
  if arch_width_define:
    if not gyp_defines:
      gyp_defines = arch_width_define
    else:
      if 'skia_arch_width' in gyp_defines.keys():
        raise ValueError('Cannot define skia_arch_width; it is derived from '
                         'the provided arch type.')
      gyp_defines.update(arch_width_define)
  return gyp_defines


# Types of builders to be used below.
class BaseBuilder:

  def _create(self, helper, do_upload_render_results, do_upload_bench_results,
              is_trybot=False):
    """Internal method used by create() to set up a builder.

    Args:
        helper: instance of utils.SkiaHelper
        do_upload_render_results: bool; whether the builder should upload its
            render results.
        do_upload_bench_results: bool; whether the builder should upload its
            bench results.
        is_trybot: bool; whether or not the builder is a trybot.
    """
    builder_name = builder_name_schema.BuilderNameFromObject(self, is_trybot)
    if is_trybot:
      # Override the scheduler for trybots.
      scheduler_name = utils.TRY_SCHEDULERS_STR
    else:
      scheduler_name = self.scheduler

    helper.Builder(builder_name, 'f_%s' % builder_name,
                   scheduler=scheduler_name)
    helper.Factory('f_%s' % builder_name, self.factory_type(
        builder_name=builder_name,
        do_patch_step=is_trybot,
        do_upload_render_results=do_upload_render_results,
        do_upload_bench_results=do_upload_bench_results,
        **self.factory_args
    ).Build(**({'role': self.role} if hasattr(self, 'role') else {})))

  def create(self, helper, do_upload_render_results, do_upload_bench_results,
             do_trybots=True):
    """Sets up a builder based on this configuration object.

    Args:
        helper: instance of utils.SkiaHelper
        do_upload_render_results: bool; whether the builder should upload its
            render results.
        do_upload_bench_results: bool; whether the builder should upload its
            bench results.
        is_trybot: bool; whether or not to create an associated try builder.
    """
    self._create(helper, do_upload_render_results, do_upload_bench_results,
                 is_trybot=False)
    if do_trybots:
      self._create(helper, do_upload_render_results, do_upload_bench_results,
                   is_trybot=True)


BuilderConfig = collections.namedtuple(
    'Builder',
    ['role', 'os', 'model', 'gpu', 'arch', 'configuration', 'extra_config',
     'gyp_defines', 'factory_type', 'target_platform', 'scheduler',
     'extra_factory_args'])


CompileBuilderConfig = collections.namedtuple(
    'CompileBuilder',
    ['os', 'compiler', 'configuration', 'target_arch', 'extra_config',
     'gyp_defines', 'warnings_as_errors', 'factory_type', 'target_platform',
     'scheduler', 'extra_factory_args'])


HousekeepingBuilderConfig = collections.namedtuple(
    'HousekeepingBuilder',
    ['frequency', 'extra_config', 'factory_type', 'target_platform',
     'scheduler', 'extra_factory_args'])


CanaryBuilderConfig = collections.namedtuple(
    'CanaryBuilder',
    ['project', 'os', 'compiler', 'target_arch', 'configuration', 'flavor',
     'build_subdir', 'gyp_defines', 'factory_type', 'target_platform',
     'scheduler', 'extra_factory_args'])


class Builder(BaseBuilder, BuilderConfig):

  @property
  def factory_args(self):
    args = {
        'configuration': self.configuration,
        'gyp_defines': get_gyp_defines(self.arch, self.gyp_defines),
        'target_platform': self.target_platform,
        'perf_output_basedir':
            (PERF_OUTPUT_BASEDIR[self.target_platform]
             if self.role == 'Perf' else None),
    }
    args.update(self.extra_factory_args)
    return args


class CompileBuilder(BaseBuilder, CompileBuilderConfig):

  role = builder_name_schema.BUILDER_ROLE_BUILD

  @property
  def factory_args(self):
    args = {
        'configuration': self.configuration,
        'gyp_defines': get_gyp_defines(self.target_arch, self.gyp_defines),
        'target_platform': self.target_platform,
        'compile_warnings_as_errors': self.warnings_as_errors,
    }
    args.update(self.extra_factory_args)
    return args


class HousekeepingBuilder(BaseBuilder, HousekeepingBuilderConfig):

  role = builder_name_schema.BUILDER_ROLE_HOUSEKEEPER

  @property
  def factory_args(self):
    args = {'target_platform': self.target_platform}
    args.update(self.extra_factory_args)
    return args


class CanaryBuilder(BaseBuilder, CanaryBuilderConfig):

  role = builder_name_schema.BUILDER_ROLE_CANARY

  @property
  def factory_args(self):
    args = {
        'gyp_defines': eval(self.gyp_defines or 'None'),
        'target_platform': self.target_platform,
        'build_subdir': self.build_subdir,
    }
    args.update(self.extra_factory_args)
    return args


def setup_builders_from_config_list(builder_specs, helper,
                                    do_upload_render_results,
                                    do_upload_bench_results, builder_format):
  """Takes a list of tuples describing builders and creates actual builders.

  Args:
      builder_specs: list of tuples following the one of the above formats.
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
      builder_format: one of the above formats.
  """
  for builder_tuple in sorted(builder_specs):
    builder = builder_format(*builder_tuple)
    builder.create(helper, do_upload_render_results, do_upload_bench_results)



def setup_test_and_perf_builders(helper, do_upload_render_results,
                                 do_upload_bench_results):
  """Set up the Test and Perf builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
  """
  #
  #                            TEST AND PERF BUILDERS
  #
  #    Role,   OS,            Model,        GPU,           Arch,     Config,    Extra Config,   GYP_DEFS,  Factory,   Target, Scheduler,       Extra Args
  #
  builder_specs = [
      ('Test', 'Ubuntu12',    'ShuttleA',   'GTX660',      'x86',    'Debug',   None,           None,      f_factory, LINUX,  S_PERCOMMIT,     {}),
      ('Test', 'Ubuntu12',    'ShuttleA',   'GTX660',      'x86',    'Release', None,           None,      f_factory, LINUX,  S_PERCOMMIT,     {}),
      ('Perf', 'Ubuntu12',    'ShuttleA',   'GTX660',      'x86',    'Release', None,           None,      f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Test', 'Ubuntu12',    'ShuttleA',   'GTX660',      'x86_64', 'Debug',   None,           None,      f_factory, LINUX,  S_PERCOMMIT,     {}),
      ('Test', 'Ubuntu12',    'ShuttleA',   'GTX660',      'x86_64', 'Release', None,           None,      f_factory, LINUX,  S_PERCOMMIT,     {}),
      ('Perf', 'Ubuntu12',    'ShuttleA',   'GTX660',      'x86_64', 'Release', None,           None,      f_factory, LINUX,  S_PERCOMMIT,     {}),
      ('Test', 'Ubuntu12',    'ShuttleA',   'GTX550Ti',    'x86_64', 'Debug',   'ZeroGPUCache', None,      f_factory, LINUX,  S_PERCOMMIT,     {}),
      ('Test', 'Ubuntu12',    'ShuttleA',   'GTX550Ti',    'x86_64', 'Release', 'Valgrind',     VALGRIND,  f_factory, LINUX,  S_PERCOMMIT,     {'flavor': 'valgrind'}),
      ('Test', 'Ubuntu13.10', 'GCE',        'NoGPU',       'x86_64', 'Debug',   None,      NO_GPU,         f_factory, LINUX,  S_PERCOMMIT,     {}),
      ('Test', 'Ubuntu13.10', 'GCE',        'NoGPU',       'x86_64', 'Release', 'Shared',  NO_GPU_SHARED,  f_factory, LINUX,  S_PERCOMMIT,     {}),
      ('Test', 'Ubuntu13.10', 'GCE',        'NoGPU',       'x86_64', 'Debug',   'ASAN',         NO_GPU,    f_xsan,    LINUX,  S_PERCOMMIT,     {'sanitizer': 'address'}),
      ('Test', 'Ubuntu13.10', 'GCE',        'NoGPU',       'x86_64', 'Release', 'TSAN',         NO_GPU,    f_xsan,    LINUX,  S_PERCOMMIT,     {'sanitizer': 'thread'}),
      ('Test', 'Mac10.6',     'MacMini4.1', 'GeForce320M', 'x86_64', 'Debug',   None,           None,      f_factory, MAC,    S_PERCOMMIT,     {}),
      ('Test', 'Mac10.6',     'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           None,      f_factory, MAC,    S_PERCOMMIT,     {}),
      ('Perf', 'Mac10.6',     'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           None,      f_factory, MAC,    S_PERCOMMIT,     {}),
      ('Test', 'Mac10.7',     'MacMini4.1', 'GeForce320M', 'x86_64', 'Debug',   None,           None,      f_factory, MAC,    S_PERCOMMIT,     {}),
      ('Test', 'Mac10.7',     'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           None,      f_factory, MAC,    S_PERCOMMIT,     {}),
      ('Perf', 'Mac10.7',     'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           None,      f_factory, MAC,    S_PERCOMMIT,     {}),
      ('Test', 'Mac10.8',     'MacMini4.1', 'GeForce320M', 'x86_64', 'Debug',   None,           None,      f_factory, MAC,    S_PERCOMMIT,     {}),
      ('Test', 'Mac10.8',     'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           PDFVIEWER, f_factory, MAC,    S_PERCOMMIT,     {}),
      ('Perf', 'Mac10.8',     'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           PDFVIEWER, f_factory, MAC,    S_PERCOMMIT,     {}),
      ('Test', 'Win7',        'ShuttleA',   'HD2000',      'x86',    'Debug',   None,           GYP_WIN7,  f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Test', 'Win7',        'ShuttleA',   'HD2000',      'x86',    'Release', None,           GYP_WIN7,  f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Perf', 'Win7',        'ShuttleA',   'HD2000',      'x86',    'Release', None,           GYP_WIN7,  f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Test', 'Win7',        'ShuttleA',   'HD2000',      'x86_64', 'Debug',   None,           GYP_WIN7,  f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Test', 'Win7',        'ShuttleA',   'HD2000',      'x86_64', 'Release', None,           GYP_WIN7,  f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Perf', 'Win7',        'ShuttleA',   'HD2000',      'x86_64', 'Release', None,           GYP_WIN7,  f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Test', 'Win7',        'ShuttleA',   'HD2000',      'x86',    'Debug',   'ANGLE',        GYP_ANGLE, f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Test', 'Win7',        'ShuttleA',   'HD2000',      'x86',    'Release', 'ANGLE',        GYP_ANGLE, f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Perf', 'Win7',        'ShuttleA',   'HD2000',      'x86',    'Release', 'ANGLE',        GYP_ANGLE, f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Test', 'Win7',        'ShuttleA',   'HD2000',      'x86',    'Debug',   'GDI',          GYP_GDI,   f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Test', 'Win7',        'ShuttleA',   'HD2000',      'x86',    'Release', 'GDI',          GYP_GDI,   f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Perf', 'Win7',        'ShuttleA',   'HD2000',      'x86',    'Release', 'GDI',          GYP_GDI,   f_factory, WIN32,  S_PERCOMMIT,     {}),
      ('Test', 'Win8',        'ShuttleA',   'GTX660',      'x86',    'Debug',   None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Test', 'Win8',        'ShuttleA',   'GTX660',      'x86',    'Release', None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Perf', 'Win8',        'ShuttleA',   'GTX660',      'x86',    'Release', None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Test', 'Win8',        'ShuttleA',   'GTX660',      'x86_64', 'Debug',   None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Test', 'Win8',        'ShuttleA',   'GTX660',      'x86_64', 'Release', None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Perf', 'Win8',        'ShuttleA',   'GTX660',      'x86_64', 'Release', None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Test', 'Win8',        'ShuttleA',   'HD7770',      'x86',    'Debug',   None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Test', 'Win8',        'ShuttleA',   'HD7770',      'x86',    'Release', None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Perf', 'Win8',        'ShuttleA',   'HD7770',      'x86',    'Release', None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Test', 'Win8',        'ShuttleA',   'HD7770',      'x86_64', 'Debug',   None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Test', 'Win8',        'ShuttleA',   'HD7770',      'x86_64', 'Release', None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Perf', 'Win8',        'ShuttleA',   'HD7770',      'x86_64', 'Release', None,           GYP_WIN8,  f_factory, WIN32,  S_PERCOMMIT,     {'build_targets': ['most']}),
      ('Test', 'ChromeOS',    'Alex',       'GMA3150',     'x86',    'Debug',   None,           None,      f_cros,    LINUX,  S_PERCOMMIT,     {'board': 'x86-alex'}),
      ('Test', 'ChromeOS',    'Alex',       'GMA3150',     'x86',    'Release', None,           None,      f_cros,    LINUX,  S_PERCOMMIT,     {'board': 'x86-alex'}),
      ('Perf', 'ChromeOS',    'Alex',       'GMA3150',     'x86',    'Release', None,           None,      f_cros,    LINUX,  S_PERCOMMIT,     {'board': 'x86-alex'}),
      ('Test', 'ChromeOS',    'Link',       'HD4000',      'x86_64', 'Debug',   None,           None,      f_cros,    LINUX,  S_PERCOMMIT,     {'board': 'link'}),
      ('Test', 'ChromeOS',    'Link',       'HD4000',      'x86_64', 'Release', None,           None,      f_cros,    LINUX,  S_PERCOMMIT,     {'board': 'link'}),
      ('Perf', 'ChromeOS',    'Link',       'HD4000',      'x86_64', 'Release', None,           None,      f_cros,    LINUX,  S_PERCOMMIT,     {'board': 'link'}),
      ('Test', 'ChromeOS',    'Daisy',      'MaliT604',    'Arm7',   'Debug',   None,           None,      f_cros,    LINUX,  S_PERCOMMIT,     {'board': 'daisy'}),
      ('Test', 'ChromeOS',    'Daisy',      'MaliT604',    'Arm7',   'Release', None,           None,      f_cros,    LINUX,  S_PERCOMMIT,     {'board': 'daisy'}),
      ('Perf', 'ChromeOS',    'Daisy',      'MaliT604',    'Arm7',   'Release', None,           None,      f_cros,    LINUX,  S_PERCOMMIT,     {'board': 'daisy'}),
  ]

  setup_builders_from_config_list(builder_specs, helper,
                                  do_upload_render_results,
                                  do_upload_bench_results, Builder)


def setup_canaries(helper, do_upload_render_results, do_upload_bench_results):
  """Set up the Canary builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
  """
  # Targets which we think are important for the Chrome canaries. Discussion:
  # https://code.google.com/p/skia/issues/detail?id=2227
  chrome_build_targets = [
      'chrome', 'base_unittests', 'cacheinvalidation_unittests',
      'cc_unittests', 'chromedriver_unittests', 'components_unittests',
      'content_unittests', 'crypto_unittests', 'google_apis_unittests',
      'gpu_unittests', 'ipc_tests', 'jingle_unittests', 'media_unittests',
      'net_unittests', 'ppapi_unittests', 'printing_unittests',
      'remoting_unittests', 'sql_unittests', 'sync_unit_tests', 'ui_unittests',
      'unit_tests', 'browser_tests', 'content_browsertests',
      'interactive_ui_tests', 'sync_integration_tests'
  ]
  #
  #                          CANARY BUILDERS
  #
  #    Project,  OS,            Compiler,Arch,     Configuration,   Flavor,  Workdir,GYP_DEFINES,Factory,  Target, Scheduler,   Extra Args
  #
  canaries = [
      ('Chrome', 'Ubuntu13.10', 'Ninja', 'x86_64', 'DRT',            None,     'src',  None,       f_drt,    LINUX,  S_PERCOMMIT, {'path_to_skia': ['third_party', 'skia']}),
      ('Chrome', 'Ubuntu13.10', 'Ninja', 'x86_64', 'ToT',            'chrome', 'src',  None,       f_canary, LINUX,  S_PERCOMMIT, {'flavor': 'chrome', 'build_targets': chrome_build_targets, 'path_to_skia': ['third_party', 'skia']}),
      ('Chrome', 'Win',         'Ninja', 'x86',    'SharedLib_ToT',  'chrome', 'src',  GYP_SHARED, f_canary, WIN32,  S_PERCOMMIT, {'flavor': 'chrome', 'build_targets': chrome_build_targets, 'path_to_skia': ['third_party', 'skia']}),
  ]

  setup_builders_from_config_list(canaries, helper, do_upload_render_results,
                                  do_upload_bench_results, CanaryBuilder)


def setup_all_builders(helper, do_upload_render_results,
                       do_upload_bench_results):
  """Set up all builders for this master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
  """
  setup_test_and_perf_builders(
      helper=helper,
      do_upload_render_results=do_upload_render_results,
      do_upload_bench_results=do_upload_bench_results)
  setup_canaries(helper=helper,
                 do_upload_render_results=do_upload_render_results,
                 do_upload_bench_results=do_upload_bench_results)


def create_schedulers_and_builders(config, active_master, cfg,
                                   builder_setup_func=setup_all_builders):
  """Create the Schedulers and Builders.

  Args:
      config: buildbot config module.
      active_master: class of the current build master.
      cfg: dict; configuration dict for the build master.
      builder_setup_func: function to call which sets up the builders for this
          master. Defaults to setup_all_builders.
  """
  helper = utils.SkiaHelper(defaults)

  # Default (per-commit) Scheduler for Skia.
  helper.Scheduler(S_PERCOMMIT)

  # Nightly Scheduler for Skia. The buildbot master follows UTC.
  # Setting it to 3AM UTC (10 PM EST).
  helper.PeriodicScheduler(S_NIGHTLY, minute=0, hour=3)

  # Scheduler which fires every 15 minutes.
  frequency_mins = 15
  minutes = [i*frequency_mins for i in xrange(60/frequency_mins)]
  helper.PeriodicScheduler(S_15MINS, minute=minutes)

  # Schedulers for Skia trybots.
  helper.TryJobSubversion(utils.TRY_SCHEDULER_SVN)
  helper.TryJobRietveld(utils.TRY_SCHEDULER_RIETVELD)

  # Call the passed-in builder setup function.
  builder_setup_func(
      helper=helper,
      # TODO(epoger): Maybe pass the following "tweaks" as a single object,
      # rather than requiring us to list them all out?
      #
      # To resolve https://code.google.com/p/skia/issues/detail?id=2432
      # ('buildbots: don't run UpdateScripts if running on non-production
      # master'), I wanted to add a third boolean flag (do_update_scripts),
      # but I saw that I would need to add it in so many places I was
      # discouraged.  If the merged all the flags into a single object it
      # would be easier to add more flags.
      do_upload_render_results=active_master.do_upload_render_results,
      do_upload_bench_results=active_master.do_upload_bench_results)

  return helper.Update(cfg)
