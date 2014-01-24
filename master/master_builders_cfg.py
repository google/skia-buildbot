# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

#pylint: disable=C0301

from skia_master_scripts.android_factory import AndroidFactory as f_android
from skia_master_scripts.canary_factory import CanaryFactory as f_canary
from skia_master_scripts.chromeos_factory import ChromeOSFactory as f_cros
from skia_master_scripts.deps_roll_factory import DepsRollFactory as f_deps
from skia_master_scripts.drt_canary_factory import DRTCanaryFactory as f_drt
from skia_master_scripts.factory import SkiaFactory as f_factory
from skia_master_scripts.housekeeping_percommit_factory \
    import HouseKeepingPerCommitFactory as f_percommit
from skia_master_scripts.housekeeping_periodic_factory \
    import HouseKeepingPeriodicFactory as f_periodic
from skia_master_scripts.ios_factory import iOSFactory as f_ios
from skia_master_scripts.nacl_factory import NaClFactory as f_nacl
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


defaults = {}


ARCH_TO_GYP_DEFINE = {
  'x86': {'skia_arch_width': '32'},
  'x86_64': {'skia_arch_width': '64'},
  'Arm7': {'skia_arch_width': '32'},
  'Mips': None,
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
GYP_DW = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
               'qt_sdk': 'C:/Qt/4.8.5/',
               'skia_directwrite': '1'})
GYP_EXC = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                'qt_sdk': 'C:/Qt/4.8.5/',
                'skia_win_exceptions': '1'})
GYP_10_6 = repr({'skia_osx_sdkroot': 'macosx10.6'})
GYP_IOS = repr({'skia_os': 'ios'})
NO_GPU = repr({'skia_gpu': '0'})
CLANG = repr({'skia_clang_build': '1'})
VALGRIND = repr({'skia_release_optimization_level': '1'})
PDFVIEWER = repr({'skia_run_pdfviewer_in_gm': '1'})
GYP_NVPR = repr({'skia_nv_path_rendering': '1'})
NVPR_WIN8 = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                  'qt_sdk': 'C:/Qt/Qt5.1.0/5.1.0/msvc2012_64/',
                  'skia_nv_path_rendering': '1'})
GYP_SHARED = repr({'component': 'shared_library'})


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
  def _create(self, helper, do_upload_results, is_trybot=False):
    """Internal method used by create() to set up a builder.

    Args:
        helper: instance of utils.SkiaHelper
        do_upload_results: bool; whether the builder should upload its results.
        is_trybot: bool; whether or not the builder is a trybot.
    """
    builder_name = builder_name_schema.BuilderNameFromObject(self, is_trybot)
    if is_trybot:
      scheduler_name = utils.TRY_SCHEDULERS_STR
    else:
      scheduler_name = 'skia_rel'

    helper.Builder(builder_name, 'f_%s' % builder_name,
                   scheduler=scheduler_name)
    helper.Factory('f_%s' % builder_name, self.factory_type(
        builder_name=builder_name,
        do_patch_step=is_trybot,
        do_upload_results=do_upload_results,
        **self.factory_args
    ).Build(**({'role': self.role} if hasattr(self, 'role') else {})))

  def create(self, helper, do_upload_results, do_trybots=True):
    """Sets up a builder based on this configuration object.

    Args:
        helper: instance of utils.SkiaHelper
        do_upload_results: bool; whether the builder should upload its results.
        is_trybot: bool; whether or not to create an associated try builder.
    """
    self._create(helper, do_upload_results, is_trybot=False)
    if do_trybots:
      self._create(helper, do_upload_results, is_trybot=True)


BuilderConfig = collections.namedtuple(
    'Builder',
    ['role', 'os', 'model', 'gpu', 'arch', 'configuration', 'extra_config',
     'gyp_defines', 'factory_type', 'target_platform', 'extra_factory_args'])


CompileBuilderConfig = collections.namedtuple(
    'CompileBuilder',
    ['os', 'compiler', 'configuration', 'target_arch', 'extra_config',
     'gyp_defines', 'warnings_as_errors', 'factory_type', 'target_platform',
     'extra_factory_args'])


HousekeepingBuilderConfig = collections.namedtuple(
    'HousekeepingBuilder',
    ['frequency', 'scheduler', 'extra_config', 'factory_type',
     'target_platform'])


CanaryBuilderConfig = collections.namedtuple(
    'CanaryBuilder',
    ['project', 'os', 'compiler', 'target_arch', 'configuration', 'flavor',
     'build_subdir', 'gyp_defines', 'factory_type', 'target_platform',
     'extra_factory_args'])


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
    return {'target_platform': self.target_platform}


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
                                    do_upload_results, builder_format):
  """Takes a list of tuples describing builders and creates actual builders.

  Args:
      builder_specs: list of tuples following the one of the above formats.
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their
          results.
      builder_format: one of the above formats.
  """
  for builder_tuple in sorted(builder_specs):
    builder = builder_format(*builder_tuple)
    builder.create(helper, do_upload_results)



def setup_test_and_perf_builders(helper, do_upload_results):
  """Set up the Test and Perf builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  #
  #                            TEST AND PERF BUILDERS
  #
  #    Role,   OS,         Model,        GPU,           Arch,     Config,    Extra Config,   GYP_DEFS,  Factory,   Target,Extra Args
  #
  builder_specs = [
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86',    'Debug',   None,           None,      f_factory, LINUX, {}),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86',    'Release', None,           None,      f_factory, LINUX, {}),
      ('Perf', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86',    'Release', None,           None,      f_factory, LINUX, {}),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Debug',   None,           None,      f_factory, LINUX, {}),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Debug',   'ZeroGPUCache', None,      f_factory, LINUX, {}),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Release', None,           None,      f_factory, LINUX, {}),
      ('Perf', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Release', None,           None,      f_factory, LINUX, {}),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Release', 'Valgrind',     VALGRIND,  f_factory, LINUX, {'flavor': 'valgrind'}),
      ('Test', 'Ubuntu12', 'ShuttleA',   'NoGPU',       'x86_64', 'Debug',   None,           NO_GPU,    f_factory, LINUX, {}),
      ('Test', 'Ubuntu13', 'ShuttleA',   'HD2000',      'x86_64', 'Debug',   'ASAN',         None,      f_xsan,    LINUX, {'sanitizer': 'address'}),
      ('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86',    'Debug',   None,           GYP_10_6,  f_factory, MAC,   {}),
      ('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           GYP_10_6,  f_factory, MAC,   {}),
      ('Perf', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           GYP_10_6,  f_factory, MAC,   {}),
      ('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Debug',   None,           GYP_10_6,  f_factory, MAC,   {}),
      ('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           GYP_10_6,  f_factory, MAC,   {}),
      ('Perf', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           GYP_10_6,  f_factory, MAC,   {}),
      ('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86',    'Debug',   None,           None,      f_factory, MAC,   {}),
      ('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           None,      f_factory, MAC,   {}),
      ('Perf', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           None,      f_factory, MAC,   {}),
      ('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Debug',   None,           None,      f_factory, MAC,   {}),
      ('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           None,      f_factory, MAC,   {}),
      ('Perf', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           None,      f_factory, MAC,   {}),
      ('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86',    'Debug',   None,           None,      f_factory, MAC,   {}),
      ('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           None,      f_factory, MAC,   {}),
      ('Perf', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           None,      f_factory, MAC,   {}),
      ('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Debug',   None,           None,      f_factory, MAC,   {}),
      ('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           PDFVIEWER, f_factory, MAC,   {}),
      ('Perf', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           PDFVIEWER, f_factory, MAC,   {}),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Debug',   None,           GYP_WIN7,  f_factory, WIN32, {}),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', None,           GYP_WIN7,  f_factory, WIN32, {}),
      ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', None,           GYP_WIN7,  f_factory, WIN32, {}),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86_64', 'Debug',   None,           GYP_WIN7,  f_factory, WIN32, {}),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86_64', 'Release', None,           GYP_WIN7,  f_factory, WIN32, {}),
      ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'x86_64', 'Release', None,           GYP_WIN7,  f_factory, WIN32, {}),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Debug',   'ANGLE',        GYP_ANGLE, f_factory, WIN32, {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', 'ANGLE',        GYP_ANGLE, f_factory, WIN32, {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}),
      ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', 'ANGLE',        GYP_ANGLE, f_factory, WIN32, {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Debug',   'DirectWrite',  GYP_DW,    f_factory, WIN32, {}),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', 'DirectWrite',  GYP_DW,    f_factory, WIN32, {}),
      ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', 'DirectWrite',  GYP_DW,    f_factory, WIN32, {}),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Debug',   None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Release', None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Perf', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Release', None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86_64', 'Debug',   None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86_64', 'Release', None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Perf', 'Win8',     'ShuttleA',   'GTX660',      'x86_64', 'Release', None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Release', 'NVPR',         NVPR_WIN8, f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'nvpr'}),
      ('Perf', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Release', 'NVPR',         NVPR_WIN8, f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'nvpr'}),
      ('Test', 'Win8',     'ShuttleA',   'HD7770',      'x86',    'Debug',   None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Test', 'Win8',     'ShuttleA',   'HD7770',      'x86',    'Release', None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Perf', 'Win8',     'ShuttleA',   'HD7770',      'x86',    'Release', None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Test', 'Win8',     'ShuttleA',   'HD7770',      'x86_64', 'Debug',   None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Test', 'Win8',     'ShuttleA',   'HD7770',      'x86_64', 'Release', None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Perf', 'Win8',     'ShuttleA',   'HD7770',      'x86_64', 'Release', None,           GYP_WIN8,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Test', 'Android',  'NexusS',     'SGX540',      'Arm7',   'Debug',   None,           None,      f_android, LINUX, {'device': 'nexus_s'}),
      ('Test', 'Android',  'NexusS',     'SGX540',      'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'nexus_s'}),
      ('Perf', 'Android',  'NexusS',     'SGX540',      'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'nexus_s'}),
      ('Test', 'Android',  'Nexus4',     'Adreno320',   'Arm7',   'Debug',   None,           None,      f_android, LINUX, {'device': 'nexus_4'}),
      ('Test', 'Android',  'Nexus4',     'Adreno320',   'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'nexus_4'}),
      ('Perf', 'Android',  'Nexus4',     'Adreno320',   'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'nexus_4'}),
      ('Test', 'Android',  'Nexus7',     'Tegra3',      'Arm7',   'Debug',   None,           None,      f_android, LINUX, {'device': 'nexus_7'}),
      ('Test', 'Android',  'Nexus7',     'Tegra3',      'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'nexus_7'}),
      ('Perf', 'Android',  'Nexus7',     'Tegra3',      'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'nexus_7'}),
      ('Test', 'Android',  'Nexus10',    'MaliT604',    'Arm7',   'Debug',   None,           None,      f_android, LINUX, {'device': 'nexus_10'}),
      ('Test', 'Android',  'Nexus10',    'MaliT604',    'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'nexus_10'}),
      ('Perf', 'Android',  'Nexus10',    'MaliT604',    'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'nexus_10'}),
      ('Test', 'Android',  'GalaxyNexus','SGX540',      'Arm7',   'Debug',   None,           None,      f_android, LINUX, {'device': 'galaxy_nexus'}),
      ('Test', 'Android',  'GalaxyNexus','SGX540',      'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'galaxy_nexus'}),
      ('Perf', 'Android',  'GalaxyNexus','SGX540',      'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'galaxy_nexus'}),
      ('Test', 'Android',  'Xoom',       'Tegra2',      'Arm7',   'Debug',   None,           None,      f_android, LINUX, {'device': 'xoom'}),
      ('Test', 'Android',  'Xoom',       'Tegra2',      'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'xoom'}),
      ('Perf', 'Android',  'Xoom',       'Tegra2',      'Arm7',   'Release', None,           None,      f_android, LINUX, {'device': 'xoom'}),
      ('Test', 'Android',  'IntelRhb',   'SGX544',      'x86',    'Debug',   None,           None,      f_android, LINUX, {'device': 'intel_rhb'}),
      ('Test', 'Android',  'IntelRhb',   'SGX544',      'x86',    'Release', None,           None,      f_android, LINUX, {'device': 'intel_rhb'}),
      ('Perf', 'Android',  'IntelRhb',   'SGX544',      'x86',    'Release', None,           None,      f_android, LINUX, {'device': 'intel_rhb'}),
      ('Test', 'ChromeOS', 'Alex',       'GMA3150',     'x86',    'Debug',   None,           None,      f_cros,    LINUX, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}),
      ('Test', 'ChromeOS', 'Alex',       'GMA3150',     'x86',    'Release', None,           None,      f_cros,    LINUX, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}),
      ('Perf', 'ChromeOS', 'Alex',       'GMA3150',     'x86',    'Release', None,           None,      f_cros,    LINUX, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}),
      ('Test', 'ChromeOS', 'Link',       'HD4000',      'x86_64', 'Debug',   None,           None,      f_cros,    LINUX, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}),
      ('Test', 'ChromeOS', 'Link',       'HD4000',      'x86_64', 'Release', None,           None,      f_cros,    LINUX, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}),
      ('Perf', 'ChromeOS', 'Link',       'HD4000',      'x86_64', 'Release', None,           None,      f_cros,    LINUX, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}),
      ('Test', 'ChromeOS', 'Daisy',      'MaliT604',    'Arm7',   'Debug',   None,           None,      f_cros,    LINUX, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}),
      ('Test', 'ChromeOS', 'Daisy',      'MaliT604',    'Arm7',   'Release', None,           None,      f_cros,    LINUX, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}),
      ('Perf', 'ChromeOS', 'Daisy',      'MaliT604',    'Arm7',   'Release', None,           None,      f_cros,    LINUX, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}),
  ]

  setup_builders_from_config_list(builder_specs, helper,
                                  do_upload_results, Builder)


def setup_compile_builders(helper, do_upload_results):
  """Set up the Compile builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their
          results.
  """
  #
  #                            COMPILE BUILDERS
  #
  #    OS,         Compiler, Config,    Arch,    Extra Config,   GYP_DEFS,  WERR,  Factory,   Target,Extra Args
  #
  builder_specs = [
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    None,          None,      True,  f_factory, LINUX, {}),
      ('Ubuntu12', 'GCC',    'Release', 'x86',    None,          None,      True,  f_factory, LINUX, {}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', None,          None,      True,  f_factory, LINUX, {}),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', None,          None,      True,  f_factory, LINUX, {}),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Valgrind',    VALGRIND,  False, f_factory, LINUX, {'flavor': 'valgrind'}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'NoGPU',       NO_GPU,    True,  f_factory, LINUX, {}),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'NoGPU',       NO_GPU,    True,  f_factory, LINUX, {}),
      ('Ubuntu12', 'Clang',  'Debug',   'x86_64', None,          CLANG,     True,  f_factory, LINUX, {'environment_variables': {'CC': '/usr/bin/clang', 'CXX': '/usr/bin/clang++'}}),
      ('Ubuntu13', 'GCC4.8', 'Debug',   'x86_64', None,          None,      True,  f_factory, LINUX, {}),
      ('Ubuntu13', 'Clang',  'Debug',   'x86_64', 'ASAN',        None,      False, f_xsan,    LINUX, {'sanitizer': 'address'}),
      ('Ubuntu13', 'Clang',  'Debug',   'x86_64', 'TSAN',        None,      False, f_xsan,    LINUX, {'sanitizer': 'thread'}),
      ('Ubuntu12', 'GCC',    'Debug',   'NaCl',   None,          None,      True,  f_nacl,    LINUX, {}),
      ('Ubuntu12', 'GCC',    'Release', 'NaCl',   None,          None,      True,  f_nacl,    LINUX, {}),
      ('Mac10.6',  'GCC',    'Debug',   'x86',    None,          GYP_10_6,  True,  f_factory, MAC,   {}),
      ('Mac10.6',  'GCC',    'Release', 'x86',    None,          GYP_10_6,  True,  f_factory, MAC,   {}),
      ('Mac10.6',  'GCC',    'Debug',   'x86_64', None,          GYP_10_6,  False, f_factory, MAC,   {}),
      ('Mac10.6',  'GCC',    'Release', 'x86_64', None,          GYP_10_6,  False, f_factory, MAC,   {}),
      ('Mac10.7',  'Clang',  'Debug',   'x86',    None,          None,      True,  f_factory, MAC,   {}),
      ('Mac10.7',  'Clang',  'Release', 'x86',    None,          None,      True,  f_factory, MAC,   {}),
      ('Mac10.7',  'Clang',  'Debug',   'x86_64', None,          None,      False, f_factory, MAC,   {}),
      ('Mac10.7',  'Clang',  'Release', 'x86_64', None,          None,      False, f_factory, MAC,   {}),
      ('Mac10.8',  'Clang',  'Debug',   'x86',    None,          None,      True,  f_factory, MAC,   {}),
      ('Mac10.8',  'Clang',  'Release', 'x86',    None,          None,      True,  f_factory, MAC,   {}),
      ('Mac10.8',  'Clang',  'Debug',   'x86_64', None,          None,      False, f_factory, MAC,   {}),
      ('Mac10.8',  'Clang',  'Release', 'x86_64', None,          PDFVIEWER, False, f_factory, MAC,   {}),
      ('Win7',     'VS2010', 'Debug',   'x86',    None,          GYP_WIN7,  True,  f_factory, WIN32, {}),
      ('Win7',     'VS2010', 'Release', 'x86',    None,          GYP_WIN7,  True,  f_factory, WIN32, {}),
      ('Win7',     'VS2010', 'Debug',   'x86_64', None,          GYP_WIN7,  False, f_factory, WIN32, {}),
      ('Win7',     'VS2010', 'Release', 'x86_64', None,          GYP_WIN7,  False, f_factory, WIN32, {}),
      ('Win7',     'VS2010', 'Debug',   'x86',    'ANGLE',       GYP_ANGLE, True,  f_factory, WIN32, {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}),
      ('Win7',     'VS2010', 'Release', 'x86',    'ANGLE',       GYP_ANGLE, True,  f_factory, WIN32, {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}),
      ('Win7',     'VS2010', 'Debug',   'x86',    'DirectWrite', GYP_DW,    False, f_factory, WIN32, {}),
      ('Win7',     'VS2010', 'Release', 'x86',    'DirectWrite', GYP_DW,    False, f_factory, WIN32, {}),
      ('Win7',     'VS2010', 'Debug',   'x86',    'Exceptions',  GYP_EXC,   False, f_factory, WIN32, {}),
      ('Win8',     'VS2012', 'Debug',   'x86',    None,          GYP_WIN8,  True,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Win8',     'VS2012', 'Release', 'x86',    None,          GYP_WIN8,  True,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Win8',     'VS2012', 'Debug',   'x86_64', None,          GYP_WIN8,  False, f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Win8',     'VS2012', 'Release', 'x86_64', None,          GYP_WIN8,  False, f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Win8',     'VS2012', 'Release', 'x86',    'NVPR',        NVPR_WIN8, True,  f_factory, WIN32, {'build_targets': ['most'], 'bench_pictures_cfg': 'nvpr'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'NexusS',      None,      True,  f_android, LINUX, {'device': 'nexus_s'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'NexusS',      None,      True,  f_android, LINUX, {'device': 'nexus_s'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus4',      None,      True,  f_android, LINUX, {'device': 'nexus_4'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus4',      None,      True,  f_android, LINUX, {'device': 'nexus_4'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus7',      None,      True,  f_android, LINUX, {'device': 'nexus_7'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus7',      None,      True,  f_android, LINUX, {'device': 'nexus_7'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus10',     None,      True,  f_android, LINUX, {'device': 'nexus_10'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus10',     None,      True,  f_android, LINUX, {'device': 'nexus_10'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'GalaxyNexus', None,      True,  f_android, LINUX, {'device': 'galaxy_nexus'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'GalaxyNexus', None,      True,  f_android, LINUX, {'device': 'galaxy_nexus'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Xoom',        None,      True,  f_android, LINUX, {'device': 'xoom'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Xoom',        None,      True,  f_android, LINUX, {'device': 'xoom'}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'IntelRhb',    None,      True,  f_android, LINUX, {'device': 'intel_rhb'}),
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'IntelRhb',    None,      True,  f_android, LINUX, {'device': 'intel_rhb'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Mips',   'Mips',        None,      True,  f_android, LINUX, {'device': 'mips'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'NvidiaLogan', GYP_NVPR,  True,  f_android, LINUX, {'device': 'nvidia_logan'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'NvidiaLogan', GYP_NVPR,  True,  f_android, LINUX, {'device': 'nvidia_logan'}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'Alex',        None,      True,  f_cros,    LINUX, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'Alex',        None,      True,  f_cros,    LINUX, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'Link',        None,      True,  f_cros,    LINUX, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Link',        None,      True,  f_cros,    LINUX, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Daisy',       None,      True,  f_cros,    LINUX, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Daisy',       None,      True,  f_cros,    LINUX, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}),
      ('Mac10.7',  'Clang',  'Debug',   'Arm7',   'iOS',         GYP_IOS,   True,  f_ios,     MAC,   {}),
      ('Mac10.7',  'Clang',  'Release', 'Arm7',   'iOS',         GYP_IOS,   True,  f_ios,     MAC,   {}),
  ]

  setup_builders_from_config_list(builder_specs, helper,
                                  do_upload_results, CompileBuilder)


def setup_housekeepers(helper, do_upload_results):
  """Set up the Housekeeping builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  #
  #                          HOUSEKEEPING BUILDERS
  #
  #   Frequency,    Scheduler,       Extra Config,Factory,     Target
  #
  housekeepers = [
      ('PerCommit', 'skia_rel',      None,        f_percommit, LINUX),
      ('Nightly',   'skia_periodic', None,        f_periodic,  LINUX),
      ('Nightly',   'skia_periodic', 'DEPSRoll',  f_deps,      LINUX),
  ]

  setup_builders_from_config_list(housekeepers, helper,
                                  do_upload_results, HousekeepingBuilder)


def setup_canaries(helper, do_upload_results):
  """Set up the Canary builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  #
  #                          CANARY BUILDERS
  #
  #    Project,  OS,         Compiler,Arch,     Configuration,Flavor,  Workdir,GYP_DEFINES,Factory,  Target,Extra Args
  #
  canaries = [
      ('Chrome', 'Ubuntu12', 'Ninja', 'x86_64', 'Default',   'chrome', 'src',  None,       f_canary, LINUX, {'flavor': 'chrome', 'build_targets': ['chrome'], 'path_to_skia': ['third_party', 'skia']}),
      ('Chrome', 'Win7',     'Ninja', 'x86',    'SharedLib', 'chrome', 'src',  GYP_SHARED, f_canary, WIN32, {'flavor': 'chrome', 'build_targets': ['chrome'], 'path_to_skia': ['third_party', 'skia']}),
      ('Chrome', 'Ubuntu12', 'Ninja', 'x86_64', 'DRT',       None,     'src',  None,       f_drt,    LINUX, {'path_to_skia': ['third_party', 'skia']}),
  ]

  setup_builders_from_config_list(canaries, helper, do_upload_results,
                                  CanaryBuilder)


def setup_all_builders(helper, do_upload_results):
  """Set up all builders for this master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  setup_compile_builders(helper=helper, do_upload_results=do_upload_results)
  setup_test_and_perf_builders(helper=helper,
                               do_upload_results=do_upload_results)
  setup_housekeepers(helper=helper, do_upload_results=do_upload_results)
  setup_canaries(helper=helper, do_upload_results=do_upload_results)


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

  # Default (per-commit) Scheduler for Skia. Only use this for builders which
  # do not care about commits outside of SKIA_PRIMARY_SUBDIRS.
  helper.AnyBranchScheduler('skia_rel', branches=utils.SKIA_PRIMARY_SUBDIRS)

  # Periodic Scheduler for Skia. The buildbot master follows UTC.
  # Setting it to 7AM UTC (2 AM EST).
  helper.PeriodicScheduler('skia_periodic', branch='trunk', minute=0, hour=7)

  # Schedulers for Skia trybots.
  helper.TryJobSubversion(utils.TRY_SCHEDULER_SVN)
  helper.TryJobRietveld(utils.TRY_SCHEDULER_RIETVELD)

  # Only upload results if we're the production master.
  do_upload_results = (active_master.do_upload_results and
                       active_master.is_production_host)

  # Call the passed-in builder setup function.
  builder_setup_func(helper=helper, do_upload_results=do_upload_results)

  return helper.Update(cfg)
