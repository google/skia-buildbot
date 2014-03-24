# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the Compile buildbot master to run.


from master_builders_cfg import CLANG, CompileBuilder, f_android, f_cros
from master_builders_cfg import f_factory, f_ios, f_nacl, f_xsan, GYP_10_6
from master_builders_cfg import GYP_ANGLE, GYP_DW, GYP_EXC, GYP_IOS
from master_builders_cfg import GYP_WIN7, GYP_WIN8, LINUX, MAC, NO_GPU
from master_builders_cfg import PDFVIEWER, S_PERCOMMIT, VALGRIND, WIN32

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
  #    OS,         Compiler, Config,    Arch,    Extra Config,   GYP_DEFS,  WERR,  Factory,   Target, Scheduler,   Extra Args
  #
  builder_specs = [
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu12', 'GCC',    'Release', 'x86',    None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Valgrind',    VALGRIND,  False, f_factory, LINUX,  S_PERCOMMIT, {'flavor': 'valgrind'}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'NoGPU',       NO_GPU,    True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'NoGPU',       NO_GPU,    True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu12', 'Clang',  'Debug',   'x86_64', None,          CLANG,     True,  f_factory, LINUX,  S_PERCOMMIT, {'environment_variables': {'CC': '/usr/bin/clang', 'CXX': '/usr/bin/clang++'}}),
      ('Ubuntu13', 'GCC4.8', 'Debug',   'x86_64', None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu13', 'GCC4.8', 'Release', 'x86_64', None,          None,      True,  f_factory, LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu13', 'Clang',  'Debug',   'x86_64', 'ASAN',        None,      False, f_xsan,    LINUX,  S_PERCOMMIT, {'sanitizer': 'address'}),
      ('Ubuntu13', 'Clang',  'Debug',   'x86_64', 'TSAN',        None,      False, f_xsan,    LINUX,  S_PERCOMMIT, {'sanitizer': 'thread'}),
      ('Ubuntu12', 'GCC',    'Debug',   'NaCl',   None,          None,      True,  f_nacl,    LINUX,  S_PERCOMMIT, {}),
      ('Ubuntu12', 'GCC',    'Release', 'NaCl',   None,          None,      True,  f_nacl,    LINUX,  S_PERCOMMIT, {}),
      ('Mac10.6',  'GCC',    'Debug',   'x86',    None,          GYP_10_6,  True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.6',  'GCC',    'Release', 'x86',    None,          GYP_10_6,  True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.6',  'GCC',    'Debug',   'x86_64', None,          GYP_10_6,  False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.6',  'GCC',    'Release', 'x86_64', None,          GYP_10_6,  False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.7',  'Clang',  'Debug',   'x86',    None,          None,      True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.7',  'Clang',  'Release', 'x86',    None,          None,      True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.7',  'Clang',  'Debug',   'x86_64', None,          None,      False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.7',  'Clang',  'Release', 'x86_64', None,          None,      False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.8',  'Clang',  'Debug',   'x86',    None,          None,      True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.8',  'Clang',  'Release', 'x86',    None,          None,      True,  f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.8',  'Clang',  'Debug',   'x86_64', None,          None,      False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Mac10.8',  'Clang',  'Release', 'x86_64', None,          PDFVIEWER, False, f_factory, MAC,    S_PERCOMMIT, {}),
      ('Win7',     'VS2010', 'Debug',   'x86',    None,          GYP_WIN7,  True,  f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win7',     'VS2010', 'Release', 'x86',    None,          GYP_WIN7,  True,  f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win7',     'VS2010', 'Debug',   'x86_64', None,          GYP_WIN7,  False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win7',     'VS2010', 'Release', 'x86_64', None,          GYP_WIN7,  False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win7',     'VS2010', 'Debug',   'x86',    'ANGLE',       GYP_ANGLE, True,  f_factory, WIN32,  S_PERCOMMIT, {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}),
      ('Win7',     'VS2010', 'Release', 'x86',    'ANGLE',       GYP_ANGLE, True,  f_factory, WIN32,  S_PERCOMMIT, {'gm_args': ['--config', 'angle'], 'bench_args': ['--config', 'ANGLE'], 'bench_pictures_cfg': 'angle'}),
      ('Win7',     'VS2010', 'Debug',   'x86',    'DirectWrite', GYP_DW,    False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win7',     'VS2010', 'Release', 'x86',    'DirectWrite', GYP_DW,    False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win7',     'VS2010', 'Debug',   'x86',    'Exceptions',  GYP_EXC,   False, f_factory, WIN32,  S_PERCOMMIT, {}),
      ('Win8',     'VS2012', 'Debug',   'x86',    None,          GYP_WIN8,  True,  f_factory, WIN32,  S_PERCOMMIT, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Win8',     'VS2012', 'Release', 'x86',    None,          GYP_WIN8,  True,  f_factory, WIN32,  S_PERCOMMIT, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Win8',     'VS2012', 'Debug',   'x86_64', None,          GYP_WIN8,  False, f_factory, WIN32,  S_PERCOMMIT, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Win8',     'VS2012', 'Release', 'x86_64', None,          GYP_WIN8,  False, f_factory, WIN32,  S_PERCOMMIT, {'build_targets': ['most'], 'bench_pictures_cfg': 'default_msaa16'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'NexusS',      None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_s'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'NexusS',      None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_s'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus4',      None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_4'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus4',      None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_4'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus7',      None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_7'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus7',      None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_7'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus10',     None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_10'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus10',     None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_10'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'GalaxyNexus', None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'galaxy_nexus'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'GalaxyNexus', None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'galaxy_nexus'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Xoom',        None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'xoom'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Xoom',        None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'xoom'}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'IntelRhb',    None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'intel_rhb'}),
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'IntelRhb',    None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'intel_rhb'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Mips',   'Mips',        None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'mips'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'NvidiaLogan', None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nvidia_logan'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'NvidiaLogan', None,      True,  f_android, LINUX,  S_PERCOMMIT, {'device': 'nvidia_logan'}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'Alex',        None,      True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'Alex',        None,      True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'x86-alex', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'Link',        None,      True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Link',        None,      True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'link', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Daisy',       None,      True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Daisy',       None,      True,  f_cros,    LINUX,  S_PERCOMMIT, {'board': 'daisy', 'bench_pictures_cfg': 'no_gpu'}),
      ('Mac10.7',  'Clang',  'Debug',   'Arm7',   'iOS',         GYP_IOS,   True,  f_ios,     MAC,    S_PERCOMMIT, {}),
      ('Mac10.7',  'Clang',  'Release', 'Arm7',   'iOS',         GYP_IOS,   True,  f_ios,     MAC,    S_PERCOMMIT, {}),
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
