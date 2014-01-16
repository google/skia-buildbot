# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

#pylint: disable=C0301

from skia_master_scripts.android_factory import AndroidFactory as f_android
from skia_master_scripts import canary_factory
from skia_master_scripts.chromeos_factory import ChromeOSFactory as f_cros
from skia_master_scripts import deps_roll_factory
from skia_master_scripts import drt_canary_factory
from skia_master_scripts.factory import SkiaFactory as f_factory
from skia_master_scripts import housekeeping_percommit_factory
from skia_master_scripts import housekeeping_periodic_factory
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


# Named tuples for easier reading from the builder configuration lists.
CompileBuilder = collections.namedtuple('CompileBuilder',
    ['os', 'compiler', 'configuration', 'target_arch', 'extra_config',
     'gyp_defines', 'warnings_as_errors', 'factory_args', 'factory_type',
     'target_platform'])
Builder = collections.namedtuple('Builder',
    ['role', 'os', 'model', 'gpu', 'arch', 'configuration', 'extra_config',
     'gyp_defines', 'gm_subdir', 'factory_args', 'factory_type',
     'target_platform'])


def setup_compile_builders_from_config_list(compile_builder_specs, helper,
                                            do_upload_results):
  """Takes a list describing Compile builders and creates actual builders.

  Args:
      compile_builder_specs: list of tuples following the CompileBuilder
          format.
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their
          results.
  """
  for compile_tuple in sorted(compile_builder_specs):
    compile_builder = CompileBuilder(*compile_tuple)
    factory_type = compile_builder.factory_type
    factory_args = compile_builder.factory_args
    target_platform = compile_builder.target_platform
    try:
      arch_width_define = ARCH_TO_GYP_DEFINE[compile_builder[3]]
    except KeyError:
      raise Exception('Unknown arch type: %s' % compile_builder[3])
    gyp_defines = eval(compile_builder.gyp_defines or 'None')
    if arch_width_define:
      if not gyp_defines:
        gyp_defines = arch_width_define
      else:
        if 'skia_arch_width' in gyp_defines.keys():
          raise ValueError('Cannot define skia_arch_width; it is derived from '
                           'the provided arch type.')
        gyp_defines.update(arch_width_define)
    utils.MakeCompileBuilderSet(
        helper=helper,
        scheduler='skia_rel',
        os=compile_builder.os,
        compiler=compile_builder.compiler,
        configuration=compile_builder.configuration,
        target_arch=compile_builder.target_arch,
        extra_config=compile_builder.extra_config,
        gyp_defines=gyp_defines,
        do_upload_results=do_upload_results,
        compile_warnings_as_errors=compile_builder.warnings_as_errors,
        factory_type=factory_type,
        target_platform=target_platform,
        **factory_args)


def setup_test_and_perf_builders_from_config_list(builder_specs, helper,
                                                  do_upload_results):
  """Takes a list describing Test and Perf builders and creates builders.

  Args:
      builder_specs: list of tuples following the Builder format.
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their
          results.
  """
  for builder_tuple in builder_specs:
    builder = Builder(*builder_tuple)
    factory_type = builder.factory_type
    factory_args = builder.factory_args
    target_platform = builder.target_platform
    try:
      arch_width_define = ARCH_TO_GYP_DEFINE[builder.arch]
    except KeyError:
      raise Exception('Unknown arch type: %s' % builder.arch)
    gyp_defines = eval(builder.gyp_defines or 'None')
    if arch_width_define:
      if not gyp_defines:
        gyp_defines = arch_width_define
      else:
        if 'skia_arch_width' in gyp_defines.keys():
          raise ValueError('Cannot define skia_arch_width; it is derived from '
                           'the provided arch type.')
        gyp_defines.update(arch_width_define)
    role = builder.role
    perf_output_basedir = None
    if role == builder_name_schema.BUILDER_ROLE_PERF:
      if target_platform == LINUX:
        perf_output_basedir = perf_output_basedir_linux
      elif target_platform == MAC:
        perf_output_basedir = perf_output_basedir_mac
      elif target_platform == WIN32:
        perf_output_basedir = perf_output_basedir_windows
    utils.MakeBuilderSet(
        helper=helper,
        role=role,
        os=builder.os,
        model=builder.model,
        gpu=builder.gpu,
        extra_config=builder.extra_config,
        configuration=builder.configuration,
        arch=builder.arch,
        gyp_defines=gyp_defines,
        factory_type=factory_type,
        target_platform=target_platform,
        gm_image_subdir=builder.gm_subdir,
        do_upload_results=do_upload_results,
        perf_output_basedir=perf_output_basedir,
        compile_warnings_as_errors=False,
        **factory_args)


def setup_test_and_perf_builders(helper, do_upload_results):
  """Set up the Test and Perf builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  #
  #                            TEST AND PERF BUILDERS
  #
  #    Role    OS          Model         GPU            Arch      Config     Extra Config    GYP_DEFS GM Subdir Factory Args
  #
  builder_specs = [
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86',    'Debug',   None,           None,      'base-shuttle_ubuntu12_ati5770', {}, f_factory, LINUX),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86',    'Release', None,           None,      'base-shuttle_ubuntu12_ati5770', {}, f_factory, LINUX),
      ('Perf', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86',    'Release', None,           None,      None, {}, f_factory, LINUX),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Debug',   None,           None,      'base-shuttle_ubuntu12_ati5770', {}, f_factory, LINUX),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Debug',   'ZeroGPUCache', None,      None, {}, f_factory, LINUX),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Release', None,           None,      'base-shuttle_ubuntu12_ati5770', {}, f_factory, LINUX),
      ('Perf', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Release', None,           None,      None, {}, f_factory, LINUX),
      ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'x86_64', 'Release', 'Valgrind',     VALGRIND,  None, {'flavor': 'valgrind'}, f_factory, LINUX),
      ('Test', 'Ubuntu12', 'ShuttleA',   'NoGPU',       'x86_64', 'Debug',   None,           NO_GPU,    'base-shuttle_ubuntu12_ati5770', {}, f_factory, LINUX),
      ('Test', 'Ubuntu13', 'ShuttleA',   'HD2000',      'x86_64', 'Debug',   'ASAN',         None,      None, {'sanitizer': 'address'}, f_xsan, LINUX),
      ('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86',    'Debug',   None,           GYP_10_6,  'base-macmini', {}, f_factory, MAC),
      ('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           GYP_10_6,  'base-macmini', {}, f_factory, MAC),
      ('Perf', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           GYP_10_6,  None, {}, f_factory, MAC),
      ('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Debug',   None,           GYP_10_6,  'base-macmini', {}, f_factory, MAC),
      ('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           GYP_10_6,  'base-macmini', {}, f_factory, MAC),
      ('Perf', 'Mac10.6',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           GYP_10_6,  None, {}, f_factory, MAC),
      ('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86',    'Debug',   None,           None,      'base-macmini-lion-float', {}, f_factory, MAC),
      ('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           None,      'base-macmini-lion-float', {}, f_factory, MAC),
      ('Perf', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           None,      None, {}, f_factory, MAC),
      ('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Debug',   None,           None,      'base-macmini-lion-float', {}, f_factory, MAC),
      ('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           None,      'base-macmini-lion-float', {}, f_factory, MAC),
      ('Perf', 'Mac10.7',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           None,      None, {}, f_factory, MAC),
      ('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86',    'Debug',   None,           None,      'base-macmini-10_8', {}, f_factory, MAC),
      ('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           None,      'base-macmini-10_8', {}, f_factory, MAC),
      ('Perf', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86',    'Release', None,           None,      None, {}, f_factory, MAC),
      ('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Debug',   None,           None,      'base-macmini-10_8', {}, f_factory, MAC),
      ('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           PDFVIEWER, 'base-macmini-10_8', {}, f_factory, MAC),
      ('Perf', 'Mac10.8',  'MacMini4.1', 'GeForce320M', 'x86_64', 'Release', None,           PDFVIEWER, None, {}, f_factory, MAC),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Debug',   None,           GYP_WIN7,  'base-shuttle-win7-intel-float', {}, f_factory, WIN32),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', None,           GYP_WIN7,  'base-shuttle-win7-intel-float', {}, f_factory, WIN32),
      ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', None,           GYP_WIN7,  None, {}, f_factory, WIN32),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86_64', 'Debug',   None,           GYP_WIN7,  'base-shuttle-win7-intel-float', {}, f_factory, WIN32),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86_64', 'Release', None,           GYP_WIN7,  'base-shuttle-win7-intel-float', {}, f_factory, WIN32),
      ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'x86_64', 'Release', None,           GYP_WIN7,  None, {}, f_factory, WIN32),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Debug',   'ANGLE',        GYP_ANGLE, 'base-shuttle-win7-intel-angle', {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}, f_factory, WIN32),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', 'ANGLE',        GYP_ANGLE, 'base-shuttle-win7-intel-angle', {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}, f_factory, WIN32),
      ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', 'ANGLE',        GYP_ANGLE, None, {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}, f_factory, WIN32),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Debug',   'DirectWrite',  GYP_DW,    'base-shuttle-win7-intel-directwrite', {}, f_factory, WIN32),
      ('Test', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', 'DirectWrite',  GYP_DW,    'base-shuttle-win7-intel-directwrite', {}, f_factory, WIN32),
      ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'x86',    'Release', 'DirectWrite',  GYP_DW,    None, {}, f_factory, WIN32),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Debug',   None,           GYP_WIN8,  'base-shuttle-win8-gtx660', {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Release', None,           GYP_WIN8,  'base-shuttle-win8-gtx660', {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Perf', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Release', None,           GYP_WIN8,  None, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86_64', 'Debug',   None,           GYP_WIN8,  'base-shuttle-win8-gtx660', {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86_64', 'Release', None,           GYP_WIN8,  'base-shuttle-win8-gtx660', {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Perf', 'Win8',     'ShuttleA',   'GTX660',      'x86_64', 'Release', None,           GYP_WIN8,  None, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Test', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Release', 'NVPR',         NVPR_WIN8, 'base-shuttle-win8-gtx660-nvpr', {'build_targets': ['most'], 'bench_pictures_cfg': 'nvpr'}, f_factory, WIN32),
      ('Perf', 'Win8',     'ShuttleA',   'GTX660',      'x86',    'Release', 'NVPR',         NVPR_WIN8, None, {'build_targets': ['most'], 'bench_pictures_cfg': 'nvpr'}, f_factory, WIN32),
      ('Test', 'Win8',     'ShuttleA',   'HD7770',      'x86',    'Debug',   None,           GYP_WIN8,  'base-shuttle-win8-hd7770', {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Test', 'Win8',     'ShuttleA',   'HD7770',      'x86',    'Release', None,           GYP_WIN8,  'base-shuttle-win8-hd7770', {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Perf', 'Win8',     'ShuttleA',   'HD7770',      'x86',    'Release', None,           GYP_WIN8,  None, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Test', 'Win8',     'ShuttleA',   'HD7770',      'x86_64', 'Debug',   None,           GYP_WIN8,  'base-shuttle-win8-hd7770', {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Test', 'Win8',     'ShuttleA',   'HD7770',      'x86_64', 'Release', None,           GYP_WIN8,  'base-shuttle-win8-hd7770', {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Perf', 'Win8',     'ShuttleA',   'HD7770',      'x86_64', 'Release', None,           GYP_WIN8,  None, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Test', 'Android',  'NexusS',     'SGX540',      'Arm7',   'Debug',   None,           None,      'base-android-nexus-s', {'device': 'nexus_s'}, f_android, LINUX),
      ('Test', 'Android',  'NexusS',     'SGX540',      'Arm7',   'Release', None,           None,      'base-android-nexus-s', {'device': 'nexus_s'}, f_android, LINUX),
      ('Perf', 'Android',  'NexusS',     'SGX540',      'Arm7',   'Release', None,           None,      None, {'device': 'nexus_s'}, f_android, LINUX),
      ('Test', 'Android',  'Nexus4',     'Adreno320',   'Arm7',   'Debug',   None,           None,      'base-android-nexus-4', {'device': 'nexus_4'}, f_android, LINUX),
      ('Test', 'Android',  'Nexus4',     'Adreno320',   'Arm7',   'Release', None,           None,      'base-android-nexus-4', {'device': 'nexus_4'}, f_android, LINUX),
      ('Perf', 'Android',  'Nexus4',     'Adreno320',   'Arm7',   'Release', None,           None,      None, {'device': 'nexus_4'}, f_android, LINUX),
      ('Test', 'Android',  'Nexus7',     'Tegra3',      'Arm7',   'Debug',   None,           None,      'base-android-nexus-7', {'device': 'nexus_7'}, f_android, LINUX),
      ('Test', 'Android',  'Nexus7',     'Tegra3',      'Arm7',   'Release', None,           None,      'base-android-nexus-7', {'device': 'nexus_7'}, f_android, LINUX),
      ('Perf', 'Android',  'Nexus7',     'Tegra3',      'Arm7',   'Release', None,           None,      None, {'device': 'nexus_7'}, f_android, LINUX),
      ('Test', 'Android',  'Nexus10',    'MaliT604',    'Arm7',   'Debug',   None,           None,      'base-android-nexus-10', {'device': 'nexus_10'}, f_android, LINUX),
      ('Test', 'Android',  'Nexus10',    'MaliT604',    'Arm7',   'Release', None,           None,      'base-android-nexus-10', {'device': 'nexus_10'}, f_android, LINUX),
      ('Perf', 'Android',  'Nexus10',    'MaliT604',    'Arm7',   'Release', None,           None,      None, {'device': 'nexus_10'}, f_android, LINUX),
      ('Test', 'Android',  'GalaxyNexus','SGX540',      'Arm7',   'Debug',   None,           None,      'base-android-galaxy-nexus', {'device': 'galaxy_nexus'}, f_android, LINUX),
      ('Test', 'Android',  'GalaxyNexus','SGX540',      'Arm7',   'Release', None,           None,      'base-android-galaxy-nexus', {'device': 'galaxy_nexus'}, f_android, LINUX),
      ('Perf', 'Android',  'GalaxyNexus','SGX540',      'Arm7',   'Release', None,           None,      None, {'device': 'galaxy_nexus'}, f_android, LINUX),
      ('Test', 'Android',  'Xoom',       'Tegra2',      'Arm7',   'Debug',   None,           None,      'base-android-xoom', {'device': 'xoom'}, f_android, LINUX),
      ('Test', 'Android',  'Xoom',       'Tegra2',      'Arm7',   'Release', None,           None,      'base-android-xoom', {'device': 'xoom'}, f_android, LINUX),
      ('Perf', 'Android',  'Xoom',       'Tegra2',      'Arm7',   'Release', None,           None,      None, {'device': 'xoom'}, f_android, LINUX),
      ('Test', 'Android',  'IntelRhb',   'SGX544',      'x86',    'Debug',   None,           None,      'base-android-intel-rhb', {'device': 'intel_rhb'}, f_android, LINUX),
      ('Test', 'Android',  'IntelRhb',   'SGX544',      'x86',    'Release', None,           None,      'base-android-intel-rhb', {'device': 'intel_rhb'}, f_android, LINUX),
      ('Perf', 'Android',  'IntelRhb',   'SGX544',      'x86',    'Release', None,           None,      None, {'device': 'intel_rhb'}, f_android, LINUX),
      ('Test', 'ChromeOS', 'Alex',       'GMA3150',     'x86',    'Debug',   None,           None,      None, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Test', 'ChromeOS', 'Alex',       'GMA3150',     'x86',    'Release', None,           None,      None, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Perf', 'ChromeOS', 'Alex',       'GMA3150',     'x86',    'Release', None,           None,      None, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Test', 'ChromeOS', 'Link',       'HD4000',      'x86_64', 'Debug',   None,           None,      None, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Test', 'ChromeOS', 'Link',       'HD4000',      'x86_64', 'Release', None,           None,      None, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Perf', 'ChromeOS', 'Link',       'HD4000',      'x86_64', 'Release', None,           None,      None, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Test', 'ChromeOS', 'Daisy',      'MaliT604',    'Arm7',   'Debug',   None,           None,      None, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Test', 'ChromeOS', 'Daisy',      'MaliT604',    'Arm7',   'Release', None,           None,      None, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Perf', 'ChromeOS', 'Daisy',      'MaliT604',    'Arm7',   'Release', None,           None,      None, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
  ]

  setup_test_and_perf_builders_from_config_list(builder_specs, helper,
                                                do_upload_results)


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
  #    OS          Compiler  Config     Arch     Extra Config    GYP_DEFS   WERR
  #
  builder_specs = [
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    None,          None,      True,  {}, f_factory, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'x86',    None,          None,      True,  {}, f_factory, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', None,          None,      True,  {}, f_factory, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', None,          None,      True,  {}, f_factory, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Valgrind',    VALGRIND,  False, {'flavor': 'valgrind'}, f_factory, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'NoGPU',       NO_GPU,    True,  {}, f_factory, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'NoGPU',       NO_GPU,    True,  {}, f_factory, LINUX),
      ('Ubuntu12', 'Clang',  'Debug',   'x86_64', None,          CLANG,     True,  {'environment_variables': {'CC': '/usr/bin/clang', 'CXX': '/usr/bin/clang++'}}, f_factory, LINUX),
      ('Ubuntu13', 'GCC4.8', 'Debug',   'x86_64', None,          None,      True,  {}, f_factory, LINUX),
      ('Ubuntu13', 'Clang',  'Debug',   'x86_64', 'ASAN',        None,      False, {'sanitizer': 'address'}, f_xsan, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'NaCl',   None,          None,      True,  {}, f_nacl, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'NaCl',   None,          None,      True,  {}, f_nacl, LINUX),
      ('Mac10.6',  'GCC',    'Debug',   'x86',    None,          GYP_10_6,  True,  {}, f_factory, MAC),
      ('Mac10.6',  'GCC',    'Release', 'x86',    None,          GYP_10_6,  True,  {}, f_factory, MAC),
      ('Mac10.6',  'GCC',    'Debug',   'x86_64', None,          GYP_10_6,  False, {}, f_factory, MAC),
      ('Mac10.6',  'GCC',    'Release', 'x86_64', None,          GYP_10_6,  False, {}, f_factory, MAC),
      ('Mac10.7',  'Clang',  'Debug',   'x86',    None,          None,      True,  {}, f_factory, MAC),
      ('Mac10.7',  'Clang',  'Release', 'x86',    None,          None,      True,  {}, f_factory, MAC),
      ('Mac10.7',  'Clang',  'Debug',   'x86_64', None,          None,      False, {}, f_factory, MAC),
      ('Mac10.7',  'Clang',  'Release', 'x86_64', None,          None,      False, {}, f_factory, MAC),
      ('Mac10.8',  'Clang',  'Debug',   'x86',    None,          None,      True,  {}, f_factory, MAC),
      ('Mac10.8',  'Clang',  'Release', 'x86',    None,          None,      True,  {}, f_factory, MAC),
      ('Mac10.8',  'Clang',  'Debug',   'x86_64', None,          None,      False, {}, f_factory, MAC),
      ('Mac10.8',  'Clang',  'Release', 'x86_64', None,          PDFVIEWER, False, {}, f_factory, MAC),
      ('Win7',     'VS2010', 'Debug',   'x86',    None,          GYP_WIN7,  True,  {}, f_factory, WIN32),
      ('Win7',     'VS2010', 'Release', 'x86',    None,          GYP_WIN7,  True,  {}, f_factory, WIN32),
      ('Win7',     'VS2010', 'Debug',   'x86_64', None,          GYP_WIN7,  False, {}, f_factory, WIN32),
      ('Win7',     'VS2010', 'Release', 'x86_64', None,          GYP_WIN7,  False, {}, f_factory, WIN32),
      ('Win7',     'VS2010', 'Debug',   'x86',    'ANGLE',       GYP_ANGLE, True,  {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}, f_factory, WIN32),
      ('Win7',     'VS2010', 'Release', 'x86',    'ANGLE',       GYP_ANGLE, True,  {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}, f_factory, WIN32),
      ('Win7',     'VS2010', 'Debug',   'x86',    'DirectWrite', GYP_DW,    False, {}, f_factory, WIN32),
      ('Win7',     'VS2010', 'Release', 'x86',    'DirectWrite', GYP_DW,    False, {}, f_factory, WIN32),
      ('Win7',     'VS2010', 'Debug',   'x86',    'Exceptions',  GYP_EXC,   False, {}, f_factory, WIN32),
      ('Win8',     'VS2012', 'Debug',   'x86',    None,          GYP_WIN8,  True,  {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Win8',     'VS2012', 'Release', 'x86',    None,          GYP_WIN8,  True,  {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Win8',     'VS2012', 'Debug',   'x86_64', None,          GYP_WIN8,  False, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Win8',     'VS2012', 'Release', 'x86_64', None,          GYP_WIN8,  False, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}, f_factory, WIN32),
      ('Win8',     'VS2012', 'Release', 'x86',    'NVPR',        NVPR_WIN8, True,  {'build_targets': ['most'], 'bench_pictures_cfg': 'nvpr'}, f_factory, WIN32),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'NexusS',      None,      True,  {'device': 'nexus_s'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'NexusS',      None,      True,  {'device': 'nexus_s'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus4',      None,      True,  {'device': 'nexus_4'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus4',      None,      True,  {'device': 'nexus_4'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus7',      None,      True,  {'device': 'nexus_7'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus7',      None,      True,  {'device': 'nexus_7'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus10',     None,      True,  {'device': 'nexus_10'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus10',     None,      True,  {'device': 'nexus_10'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'GalaxyNexus', None,      True,  {'device': 'galaxy_nexus'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'GalaxyNexus', None,      True,  {'device': 'galaxy_nexus'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Xoom',        None,      True,  {'device': 'xoom'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Xoom',        None,      True,  {'device': 'xoom'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'IntelRhb',    None,      True,  {'device': 'intel_rhb'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'IntelRhb',    None,      True,  {'device': 'intel_rhb'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'Mips',   'Mips',        None,      True,  {'device': 'mips'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'Alex',        None,      True,  {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'Alex',        None,      True,  {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'Link',        None,      True,  {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Link',        None,      True,  {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Daisy',       None,      True,  {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Daisy',       None,      True,  {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}, f_cros, LINUX),
      ('Mac10.7',  'Clang',  'Debug',   'Arm7',   'iOS',         GYP_IOS,   True,  {}, f_ios, MAC),
      ('Mac10.7',  'Clang',  'Release', 'Arm7',   'iOS',         GYP_IOS,   True,  {}, f_ios, MAC),
  ]

  setup_compile_builders_from_config_list(builder_specs, helper,
                                          do_upload_results)


def setup_housekeepers(helper, do_upload_results):
  """Set up the Housekeeping builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  # TODO(borenet): Specify the Housekeeping and Canary builders in a nice table
  # like the Build/Test/Perf builders.
  housekeepers = [
    # The Percommit housekeeper
    (builder_name_schema.MakeBuilderName(role='Housekeeper',
                                         frequency='PerCommit'),
     housekeeping_percommit_factory.HouseKeepingPerCommitFactory,
     'skia_rel'),
    # The Periodic housekeeper
    (builder_name_schema.MakeBuilderName(role='Housekeeper',
                                         frequency='Nightly'),
     housekeeping_periodic_factory.HouseKeepingPeriodicFactory,
     'skia_periodic'),
    (builder_name_schema.MakeBuilderName(role='Housekeeper',
                                         frequency='Nightly',
                                         extra_config='DEPSRoll'),
     deps_roll_factory.DepsRollFactory,
     'skia_periodic'),
  ]
  # Add the corresponding trybot builders to the above list.
  housekeepers.extend([
      (builder + builder_name_schema.BUILDER_NAME_SEP + \
       builder_name_schema.TRYBOT_NAME_SUFFIX, factory,
       utils.TRY_SCHEDULERS_STR)
      for (builder, factory, _scheduler) in housekeepers])

  # Create the builders.
  for (builder_name, factory, scheduler) in housekeepers:
    helper.Builder(builder_name, 'f_%s' % builder_name, scheduler=scheduler)
    helper.Factory('f_%s' % builder_name,
        factory(
            do_upload_results=do_upload_results,
            target_platform=LINUX,
            builder_name=builder_name,
            do_patch_step=(scheduler == utils.TRY_SCHEDULERS_STR),
        ).Build())


def setup_canaries(helper, do_upload_results):
  """Set up the Canary builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  # TODO(borenet): Specify the Housekeeping and Canary builders in a nice table
  # like the Build/Test/Perf builders.
  canaries = [
      (builder_name_schema.MakeBuilderName(role='Canary',
                                           project='Chrome',
                                           os='Ubuntu12',
                                           compiler='Ninja',
                                           target_arch='x86_64',
                                           configuration='Default'),
       LINUX,
       canary_factory.CanaryFactory,
       'skia_rel',
       {
        'build_targets': ['chrome'],
        'flavor': 'chrome',
        'build_subdir': 'src',
        'path_to_skia': ['third_party', 'skia']
       }),
      (builder_name_schema.MakeBuilderName(role='Canary',
                                           project='Chrome',
                                           os='Win7',
                                           compiler='Ninja',
                                           target_arch='x86',
                                           configuration='SharedLib'),
       WIN32,
       canary_factory.CanaryFactory,
       'skia_rel',
       {
        'build_targets': ['chrome'],
        'flavor': 'chrome',
        'build_subdir': 'src',
        'path_to_skia': ['third_party', 'skia'],
        'gyp_defines': {
          'component': 'shared_library',
        },
       }),
      (builder_name_schema.MakeBuilderName(role='Canary',
                                           project='Chrome',
                                           os='Ubuntu12',
                                           compiler='Ninja',
                                           target_arch='x86_64',
                                           configuration='DRT'),
       LINUX,
       drt_canary_factory.DRTCanaryFactory,
       'skia_rel',
       {
        'build_subdir': 'src',
        'path_to_skia': ['third_party', 'skia'],
       }),
  ]
  # Add corresponding trybot builders to the above list.
  canaries.extend([
      (builder + builder_name_schema.BUILDER_NAME_SEP + \
           builder_name_schema.TRYBOT_NAME_SUFFIX,
       target_platform,
       factory,
       utils.TRY_SCHEDULERS_STR,
       factory_args)
      for (builder, target_platform, factory, _scheduler,
           factory_args) in canaries])

  for (builder_name, target_platform, factory, scheduler,
       factory_args) in canaries:
    helper.Builder(builder_name, 'f_%s' % builder_name, scheduler=scheduler)
    helper.Factory('f_%s' % builder_name,
        factory(
            do_upload_results=do_upload_results,
            target_platform=target_platform,
            builder_name=builder_name,
            do_patch_step=(scheduler == utils.TRY_SCHEDULERS_STR),
            **factory_args
        ).Build())


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
