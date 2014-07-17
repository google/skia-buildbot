# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the Compile buildbot master to run.


#pylint: disable=C0301


from master_builders_cfg import CLANG, CompileBuilder
from master_builders_cfg import GYP_ANGLE, GYP_DW, GYP_EXC, GYP_IOS
from master_builders_cfg import GYP_WIN7, LINUX, MAC, NO_GPU
from master_builders_cfg import PDFVIEWER, S_PERCOMMIT, WIN32

from skia_master_scripts.android_factory import AndroidFactory as f_android
from skia_master_scripts.chromeos_factory import ChromeOSFactory as f_cros
from skia_master_scripts.factory import SkiaFactory as f_factory
from skia_master_scripts.ios_factory import iOSFactory as f_ios
from skia_master_scripts.nacl_factory import NaClFactory as f_nacl

import master_builders_cfg


def setup_compile_builders(helper, do_upload_render_results,
                           do_upload_bench_results):
  """Set up the Compile builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
  """
  #
  #                            COMPILE BUILDERS
  #
  #    OS,            Compiler, Config,    Arch,    Extra Config,   GYP_DEFS,  WERR,  Factory,   Target, Scheduler,   Extra Args
  #
  builder_specs = [
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'x86',    None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'x86',    None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'x86_64', None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'x86_64', None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'x86_64', 'NoGPU',       NO_GPU,    True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'x86_64', 'NoGPU',       NO_GPU,    True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu13.10', 'Clang',  'Debug',   'x86_64', None,          CLANG,     True,  f_factory, LINUX,  S_PERCOMMIT, {'environment_variables': {'CC': '/usr/bin/clang', 'CXX': '/usr/bin/clang++'}}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'NaCl',   None,          None,      True,  f_nacl,    LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'NaCl',   None,          None,      True,  f_nacl,    LINUX,  S_PERCOMMIT, {}),
      ('Mac10.7',     'Clang',  'Debug',   'x86',    None,          None,      True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.7',     'Clang',  'Release', 'x86',    None,          None,      True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.7',     'Clang',  'Debug',   'x86_64', None,          None,      False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.7',     'Clang',  'Release', 'x86_64', None,          None,      False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.8',     'Clang',  'Debug',   'x86',    None,          None,      True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.8',     'Clang',  'Release', 'x86',    None,          None,      True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.8',     'Clang',  'Debug',   'x86_64', None,          None,      False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.8',     'Clang',  'Release', 'x86_64', None,          PDFVIEWER, False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Win',         'VS2013', 'Debug',   'x86',    None,          GYP_WIN7,  True,  f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win',         'VS2013', 'Release', 'x86',    None,          GYP_WIN7,  True,  f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win',         'VS2013', 'Debug',   'x86_64', None,          GYP_WIN7,  False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win',         'VS2013', 'Release', 'x86_64', None,          GYP_WIN7,  False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win',         'VS2013', 'Debug',   'x86',    'ANGLE',       GYP_ANGLE, True,  f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win',         'VS2013', 'Release', 'x86',    'ANGLE',       GYP_ANGLE, True,  f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win',         'VS2013', 'Debug',   'x86',    'DirectWrite', GYP_DW,    False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win',         'VS2013', 'Release', 'x86',    'DirectWrite', GYP_DW,    False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win',         'VS2013', 'Debug',   'x86',    'Exceptions',  GYP_EXC,   False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'Arm7',   'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'arm_v7_thumb'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'Arm7',   'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'arm_v7_thumb'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'Arm7',   'Android_NoThumb', None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'arm_v7'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'Arm7',   'Android_NoThumb', None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'arm_v7'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'Arm7',   'Android_Neon',    None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_4'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'Arm7',   'Android_Neon',    None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_4'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'Arm7',   'Android_NoNeon',  None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'xoom'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'Arm7',   'Android_NoNeon',  None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'xoom'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'Arm64',  'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'arm64'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'Arm64',  'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'arm64'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'x86',    'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'x86'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'x86',    'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'x86'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'x86_64', 'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'x86_64'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'x86_64', 'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'x86_64'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'Mips',   'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'mips'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'Mips',   'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'mips'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'Mips64', 'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'mips64'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'Mips64', 'Android',         None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'mips64'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'MipsDSP2','Android',        None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'mips_dsp2'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'MipsDSP2','Android',        None,  True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'mips_dsp2'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'x86',    'CrOS_Alex',       None,  True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'x86-alex'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'x86',    'CrOS_Alex',       None,  True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'x86-alex'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'x86_64', 'CrOS_Link',       None,  True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'link'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'x86_64', 'CrOS_Link',       None,  True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'link'}),
      ('Ubuntu13.10', 'GCC4.8', 'Debug',   'Arm7',   'CrOS_Daisy',      None,  True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'daisy'}),
      ('Ubuntu13.10', 'GCC4.8', 'Release', 'Arm7',   'CrOS_Daisy',      None,  True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'daisy'}),
      ('Mac10.7',     'Clang',  'Debug',   'Arm7',   'iOS',          GYP_IOS,  True,  f_ios,     MAC,    S_PERCOMMIT, {}),
      ('Mac10.7',     'Clang',  'Release', 'Arm7',   'iOS',          GYP_IOS,  True,  f_ios,     MAC,    S_PERCOMMIT, {}),
  ]

  master_builders_cfg.setup_builders_from_config_list(builder_specs, helper,
                                                      do_upload_render_results,
                                                      do_upload_bench_results,
                                                      CompileBuilder)


def setup_all_builders(helper, do_upload_render_results,
                       do_upload_bench_results):
  """Set up all builders for the Compile master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
  """
  setup_compile_builders(helper, do_upload_render_results,
                         do_upload_bench_results)
