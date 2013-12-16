# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

#pylint: disable=C0301

from skia_master_scripts import android_factory
from skia_master_scripts import canary_factory
from skia_master_scripts import chromeos_factory
from skia_master_scripts import drt_canary_factory
from skia_master_scripts import factory as skia_factory
from skia_master_scripts import housekeeping_percommit_factory
from skia_master_scripts import housekeeping_periodic_factory
from skia_master_scripts import ios_factory
from skia_master_scripts import moz2d_canary_factory
from skia_master_scripts import nacl_factory
from skia_master_scripts import utils
from skia_master_scripts import xsan_factory

import builder_name_schema


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
  'NaCl': None,
}


CHROMEOS_BOARD_NAME = {
  'Alex': 'x86-alex',
  'Link': 'link',
  'Daisy': 'daisy',
}


def GetExtraFactoryArgs(compile_builder_info):
  factory_type = compile_builder_info[7]
  if factory_type == android_factory.AndroidFactory:
    # AndroidFactory requires a "device" argument.
    return {'device': utils.CapWordsToUnderscores(compile_builder_info[4])}
  elif factory_type == chromeos_factory.ChromeOSFactory:
    # ChromeOSFactory requires a "board" argument.
    try:
      return {'board': CHROMEOS_BOARD_NAME[compile_builder_info[4]],
              'bench_pictures_cfg': 'no_gpu'}
    except KeyError:
      raise Exception('Unknown board type "%s"' % compile_builder_info[4])
  elif factory_type == xsan_factory.XsanFactory:
    sanitizers = { 'ASAN': 'address', 'TSAN': 'thread' }
    return {'sanitizer': sanitizers[compile_builder_info[4]]}
  elif factory_type == skia_factory.SkiaFactory:
    # Some "normal" factories require extra arguments.
    if compile_builder_info[4] == 'ANGLE':
      return {'gm_args': ['--config', 'angle'],
              'bench_args': ['--config', 'ANGLE'],
              'bench_pictures_cfg': 'angle'}
    elif compile_builder_info[4] == 'Valgrind':
      return {'flavor': 'valgrind'}
    elif (compile_builder_info[0] == 'Ubuntu12' and
          compile_builder_info[1] == 'Clang'):
      return {'environment_variables': {'CC': '/usr/bin/clang',
                                        'CXX': '/usr/bin/clang++'}}
    elif compile_builder_info[0] == 'Win8':
      # On Win8, we build all targets at once, because of
      # https://code.google.com/p/skia/issues/detail?id=1331
      return {'build_targets': ['most']}
    else:
      return {}
  else:
    return {}


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


def setup_builders_from_config_dict(builder_specs, helper, do_upload_results):
  """Takes a dictionary describing Compile and Test/Perf builders and creates
  actual builders.

  Args:
      builder_specs: dict of the form:
              { (Compile Builder): [(Test/Perf Builder), ...], ... }
          where (Compile Builder) is a tuple:
              (os, compiler, configuration, arch, extra_config, gyp_defines,
               warnings_as_errors, target_platform, factory_class)
          and (Test/Perf Builder) is a tuble:
              (role, os, model, gpu, extra_config, gm_subdir)
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  for compile_builder in sorted(builder_specs.keys()):
    factory_type = compile_builder[7]
    factory_args = GetExtraFactoryArgs(compile_builder)
    target_platform = compile_builder[8]
    try:
      arch_width_define = ARCH_TO_GYP_DEFINE[compile_builder[3]]
    except KeyError:
      raise Exception('Unknown arch type: %s' % compile_builder[3])
    gyp_defines = eval(compile_builder[5] or 'None')
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
        os=compile_builder[0],
        compiler=compile_builder[1],
        configuration=compile_builder[2],
        target_arch=compile_builder[3],
        extra_config=compile_builder[4],
        gyp_defines=gyp_defines,
        do_upload_results=do_upload_results,
        compile_warnings_as_errors=compile_builder[6],
        factory_type=factory_type,
        target_platform=target_platform,
        **factory_args)
    for dependent_builder in builder_specs[compile_builder]:
      role = dependent_builder[0]
      perf_output_basedir = None
      if role == builder_name_schema.BUILDER_ROLE_PERF:
        if target_platform == skia_factory.TARGET_PLATFORM_LINUX:
          perf_output_basedir = perf_output_basedir_linux
        elif target_platform == skia_factory.TARGET_PLATFORM_MAC:
          perf_output_basedir = perf_output_basedir_mac
        elif target_platform == skia_factory.TARGET_PLATFORM_WIN32:
          perf_output_basedir = perf_output_basedir_windows
      utils.MakeBuilderSet(
          helper=helper,
          role=role,
          os=dependent_builder[1],
          model=dependent_builder[2],
          gpu=dependent_builder[3],
          extra_config=dependent_builder[4],
          configuration=compile_builder[2],
          arch=compile_builder[3],
          gyp_defines=gyp_defines,
          factory_type=factory_type,
          target_platform=target_platform,
          gm_image_subdir=dependent_builder[5],
          do_upload_results=do_upload_results,
          perf_output_basedir=perf_output_basedir,
          compile_warnings_as_errors=False,
          **factory_args)


def setup_primary_builders(helper, do_upload_results):
  """Set up the "primary" builders.

  These are Compile, Test, and Perf builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  # builder_specs is a dictionary whose keys are specifications for compile
  # builders and values are specifications for Test and Perf builders which will
  # eventually *depend* on those compile builders.
  builder_specs = {}
  #
  #                            COMPILE BUILDERS                                                                              TEST AND PERF BUILDERS
  #
  #    OS          Compiler  Config     Arch     Extra Config    GYP_DEFS   WERR             Role    OS          Model         GPU            Extra Config   GM Subdir
  #
  f = skia_factory.SkiaFactory
  p = skia_factory.TARGET_PLATFORM_LINUX
  builder_specs.update({
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    None,          None,      True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          'base-shuttle_ubuntu12_ati5770')],
      ('Ubuntu12', 'GCC',    'Release', 'x86',    None,          None,      True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          'base-shuttle_ubuntu12_ati5770'),
                                                                                            ('Perf', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', None,          None,      True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          'base-shuttle_ubuntu12_ati5770'),
                                                                                            ('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'ZeroGPUCache',None)],
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', None,          None,      True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          'base-shuttle_ubuntu12_ati5770'),
                                                                                            ('Perf', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          None)],
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Valgrind',    VALGRIND,  False, f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     'Valgrind',    None)],
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'NoGPU',       NO_GPU,    True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'NoGPU',       None,          'base-shuttle_ubuntu12_ati5770')],
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'NoGPU',       NO_GPU,    True,  f, p) : [],
      ('Ubuntu12', 'Clang',  'Debug',   'x86_64', None,          CLANG,     True,  f, p) : [],
      ('Ubuntu13', 'GCC4.8', 'Debug',   'x86_64', None,          None,      True,  f, p) : [],})
  f = xsan_factory.XsanFactory
  builder_specs.update({
      ('Ubuntu13', 'Clang',  'Debug',   'x86_64', 'ASAN',        None,      False, f, p) : [('Test', 'Ubuntu13', 'ShuttleA',   'HD2000',      'ASAN',        None)],
      ('Ubuntu13', 'Clang',  'Debug',   'x86_64', 'TSAN',        None,      False, f, p) : [('Test', 'Ubuntu13', 'ShuttleA',   'HD2000',      'TSAN',        None)],})
  f = nacl_factory.NaClFactory
  builder_specs.update({
      ('Ubuntu12', 'GCC',    'Debug',   'NaCl',   None,          None,      True,  f, p) : [],
      ('Ubuntu12', 'GCC',    'Release', 'NaCl',   None,          None,      True,  f, p) : [],})
  f = skia_factory.SkiaFactory
  p = skia_factory.TARGET_PLATFORM_MAC
  builder_specs.update({
      ('Mac10.6',  'GCC',    'Debug',   'x86',    None,          GYP_10_6,  True,  f, p) : [('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          'base-macmini')],
      ('Mac10.6',  'GCC',    'Release', 'x86',    None,          GYP_10_6,  True,  f, p) : [('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          'base-macmini'),
                                                                                            ('Perf', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.6',  'GCC',    'Debug',   'x86_64', None,          GYP_10_6,  False, f, p) : [('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          'base-macmini')],
      ('Mac10.6',  'GCC',    'Release', 'x86_64', None,          GYP_10_6,  False, f, p) : [('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          'base-macmini'),
                                                                                            ('Perf', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.7',  'Clang',  'Debug',   'x86',    None,          None,      True,  f, p) : [('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-lion-float')],
      ('Mac10.7',  'Clang',  'Release', 'x86',    None,          None,      True,  f, p) : [('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-lion-float'),
                                                                                            ('Perf', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.7',  'Clang',  'Debug',   'x86_64', None,          None,      False, f, p) : [('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-lion-float')],
      ('Mac10.7',  'Clang',  'Release', 'x86_64', None,          None,      False, f, p) : [('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-lion-float'),
                                                                                            ('Perf', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.8',  'Clang',  'Debug',   'x86',    None,          None,      True,  f, p) : [('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-10_8')],
      ('Mac10.8',  'Clang',  'Release', 'x86',    None,          None,      True,  f, p) : [('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-10_8'),
                                                                                            ('Perf', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.8',  'Clang',  'Debug',   'x86_64', None,          None,      False, f, p) : [('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-10_8')],
      ('Mac10.8',  'Clang',  'Release', 'x86_64', None,          PDFVIEWER, False, f, p) : [('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-10_8'),
                                                                                            ('Perf', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          None)],})
  p = skia_factory.TARGET_PLATFORM_WIN32
  builder_specs.update({
      ('Win7',     'VS2010', 'Debug',   'x86',    None,          GYP_WIN7,  True,  f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      None,          'base-shuttle-win7-intel-float')],
      ('Win7',     'VS2010', 'Release', 'x86',    None,          GYP_WIN7,  True,  f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      None,          'base-shuttle-win7-intel-float'),
                                                                                            ('Perf', 'Win7',     'ShuttleA',   'HD2000',      None,          None)],
      ('Win7',     'VS2010', 'Debug',   'x86_64', None,          GYP_WIN7,  False, f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      None,          'base-shuttle-win7-intel-float')],
      ('Win7',     'VS2010', 'Release', 'x86_64', None,          GYP_WIN7,  False, f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      None,          'base-shuttle-win7-intel-float'),
                                                                                            ('Perf', 'Win7',     'ShuttleA',   'HD2000',      None,          None)],
      ('Win7',     'VS2010', 'Debug',   'x86',    'ANGLE',       GYP_ANGLE, True,  f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      'ANGLE',       'base-shuttle-win7-intel-angle')],
      ('Win7',     'VS2010', 'Release', 'x86',    'ANGLE',       GYP_ANGLE, True,  f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      'ANGLE',       'base-shuttle-win7-intel-angle'),
                                                                                            ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'ANGLE',       None)],
      ('Win7',     'VS2010', 'Debug',   'x86',    'DirectWrite', GYP_DW,    False, f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      'DirectWrite', 'base-shuttle-win7-intel-directwrite')],
      ('Win7',     'VS2010', 'Release', 'x86',    'DirectWrite', GYP_DW,    False, f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      'DirectWrite', 'base-shuttle-win7-intel-directwrite'),
                                                                                            ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'DirectWrite', None)],
      ('Win7',     'VS2010', 'Debug',   'x86',    'Exceptions',  GYP_EXC,   False, f, p) : [],
      ('Win8',     'VS2012', 'Debug',   'x86',    None,          GYP_WIN8,  True,  f, p) : [('Test', 'Win8',     'ShuttleA',   'GTX660',      None,          'base-shuttle-win8-gtx660'),
                                                                                            ('Test', 'Win8',     'ShuttleA',   'HD7770',      None,          'base-shuttle-win8-hd7770')],
      ('Win8',     'VS2012', 'Release', 'x86',    None,          GYP_WIN8,  True,  f, p) : [('Test', 'Win8',     'ShuttleA',   'GTX660',      None,          'base-shuttle-win8-gtx660'),
                                                                                            ('Perf', 'Win8',     'ShuttleA',   'GTX660',      None,          None),
                                                                                            ('Test', 'Win8',     'ShuttleA',   'HD7770',      None,          'base-shuttle-win8-hd7770'),
                                                                                            ('Perf', 'Win8',     'ShuttleA',   'HD7770',      None,          None)],
      ('Win8',     'VS2012', 'Debug',   'x86_64', None,          GYP_WIN8,  False, f, p) : [('Test', 'Win8',     'ShuttleA',   'GTX660',      None,          'base-shuttle-win8-gtx660'),
                                                                                            ('Test', 'Win8',     'ShuttleA',   'HD7770',      None,          'base-shuttle-win8-hd7770')],
      ('Win8',     'VS2012', 'Release', 'x86_64', None,          GYP_WIN8,  False, f, p) : [('Test', 'Win8',     'ShuttleA',   'GTX660',      None,          'base-shuttle-win8-gtx660'),
                                                                                            ('Perf', 'Win8',     'ShuttleA',   'GTX660',      None,          None),
                                                                                            ('Test', 'Win8',     'ShuttleA',   'HD7770',      None,          'base-shuttle-win8-hd7770'),
                                                                                            ('Perf', 'Win8',     'ShuttleA',   'HD7770',      None,          None)],
      ('Win8',     'VS2012', 'Release', 'x86',    'NVPR',        NVPR_WIN8, True,  f, p) : [('Test', 'Win8',     'ShuttleA',   'GTX660',      'NVPR',        'base-shuttle-win8-gtx660-nvpr'),
                                                                                            ('Perf', 'Win8',     'ShuttleA',   'GTX660',      'NVPR',        None)],})
  f = android_factory.AndroidFactory
  p = skia_factory.TARGET_PLATFORM_LINUX
  builder_specs.update({
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'NexusS',      None,      True,  f, p) : [('Test', 'Android',  'NexusS',     'SGX540',      None,          'base-android-nexus-s')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'NexusS',      None,      True,  f, p) : [('Test', 'Android',  'NexusS',     'SGX540',      None,          'base-android-nexus-s'),
                                                                                            ('Perf', 'Android',  'NexusS',     'SGX540',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus4',      None,      True,  f, p) : [('Test', 'Android',  'Nexus4',     'Adreno320',   None,          'base-android-nexus-4')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus4',      None,      True,  f, p) : [('Test', 'Android',  'Nexus4',     'Adreno320',   None,          'base-android-nexus-4'),
                                                                                            ('Perf', 'Android',  'Nexus4',     'Adreno320',   None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus7',      None,      True,  f, p) : [('Test', 'Android',  'Nexus7',     'Tegra3',      None,          'base-android-nexus-7')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus7',      None,      True,  f, p) : [('Test', 'Android',  'Nexus7',     'Tegra3',      None,          'base-android-nexus-7'),
                                                                                            ('Perf', 'Android',  'Nexus7',     'Tegra3',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus10',     None,      True,  f, p) : [('Test', 'Android',  'Nexus10',    'MaliT604',    None,          'base-android-nexus-10')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus10',     None,      True,  f, p) : [('Test', 'Android',  'Nexus10',    'MaliT604',    None,          'base-android-nexus-10'),
                                                                                            ('Perf', 'Android',  'Nexus10',    'MaliT604',    None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'GalaxyNexus', None,      True,  f, p) : [('Test', 'Android',  'GalaxyNexus','SGX540',      None,          'base-android-galaxy-nexus')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'GalaxyNexus', None,      True,  f, p) : [('Test', 'Android',  'GalaxyNexus','SGX540',      None,          'base-android-galaxy-nexus'),
                                                                                            ('Perf', 'Android',  'GalaxyNexus','SGX540',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Xoom',        None,      True,  f, p) : [('Test', 'Android',  'Xoom',       'Tegra2',      None,          'base-android-xoom')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Xoom',        None,      True,  f, p) : [('Test', 'Android',  'Xoom',       'Tegra2',      None,          'base-android-xoom'),
                                                                                            ('Perf', 'Android',  'Xoom',       'Tegra2',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'IntelRhb',    None,      True,  f, p) : [('Test', 'Android',  'IntelRhb',   'SGX544',      None,          'base-android-intel-rhb')],
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'IntelRhb',    None,      True,  f, p) : [('Test', 'Android',  'IntelRhb',   'SGX544',      None,          'base-android-intel-rhb'),
                                                                                            ('Perf', 'Android',  'IntelRhb',   'SGX544',      None,          None)],})
  f = chromeos_factory.ChromeOSFactory
  builder_specs.update({
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'Alex',        None,      True,  f, p) : [('Test', 'ChromeOS', 'Alex',       'GMA3150',     None,          None)],
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'Alex',        None,      True,  f, p) : [('Test', 'ChromeOS', 'Alex',       'GMA3150',     None,          None),
                                                                                            ('Perf', 'ChromeOS', 'Alex',       'GMA3150',     None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'Link',        None,      True,  f, p) : [('Test', 'ChromeOS', 'Link',       'HD4000',      None,          None)],
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Link',        None,      True,  f, p) : [('Test', 'ChromeOS', 'Link',       'HD4000',      None,          None),
                                                                                            ('Perf', 'ChromeOS', 'Link',       'HD4000',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Daisy',       None,      True,  f, p) : [('Test', 'ChromeOS', 'Daisy',      'MaliT604',    None,          None)],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Daisy',       None,      True,  f, p) : [('Test', 'ChromeOS', 'Daisy',      'MaliT604',    None,          None),
                                                                                            ('Perf', 'ChromeOS', 'Daisy',      'MaliT604',    None,          None)],})
  f = ios_factory.iOSFactory
  p = skia_factory.TARGET_PLATFORM_MAC
  builder_specs.update({
      ('Mac10.7',  'Clang',  'Debug',   'Arm7',   'iOS',         GYP_IOS,   True,  f, p) : [],
      ('Mac10.7',  'Clang',  'Release', 'Arm7',   'iOS',         GYP_IOS,   True,  f, p) : [],})

  setup_builders_from_config_dict(builder_specs, helper, do_upload_results)


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
            target_platform=skia_factory.TARGET_PLATFORM_LINUX,
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
       skia_factory.TARGET_PLATFORM_LINUX,
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
       skia_factory.TARGET_PLATFORM_WIN32,
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
                                           project='Moz2D',
                                           os='Ubuntu12',
                                           compiler='GCC',
                                           target_arch='x86_64',
                                           configuration='Release'),
       skia_factory.TARGET_PLATFORM_LINUX,
       moz2d_canary_factory.Moz2DCanaryFactory,
       'skia_rel',
       {}),
      (builder_name_schema.MakeBuilderName(role='Canary',
                                           project='Chrome',
                                           os='Ubuntu12',
                                           compiler='Ninja',
                                           target_arch='x86_64',
                                           configuration='DRT'),
       skia_factory.TARGET_PLATFORM_LINUX,
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
  setup_primary_builders(helper=helper, do_upload_results=do_upload_results)
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
