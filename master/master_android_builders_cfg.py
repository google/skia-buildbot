# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the FYI buildbot master to run.


#pylint: disable=C0301


from skia_master_scripts.android_factory import AndroidFactory as f_android
from master_builders_cfg import LINUX, S_PERCOMMIT

import master_builders_cfg


def setup_test_and_perf_builders(helper, do_upload_render_results,
                                 do_upload_bench_results):
  """Set up all Test and Perf builders for the private master.

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
  #    Role,   OS,         Model,        GPU,           Arch,     Config,    Extra Config,   GYP_DEFS,  Factory,   Target, Scheduler,   Extra Args
  #
  builder_specs = [
      ('Test', 'Android',  'Nexus5',     'Adreno330',   'Arm7',   'Debug',   None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_5'}),
      ('Test', 'Android',  'Nexus5',     'Adreno330',   'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_5'}),
      ('Perf', 'Android',  'Nexus5',     'Adreno330',   'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_5'}),
      ('Test', 'Android',  'Nexus7',     'Tegra3',      'Arm7',   'Debug',   None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_7'}),
      ('Test', 'Android',  'Nexus7',     'Tegra3',      'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_7'}),
      ('Perf', 'Android',  'Nexus7',     'Tegra3',      'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_7'}),
      ('Test', 'Android',  'Nexus10',    'MaliT604',    'Arm7',   'Debug',   None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_10'}),
      ('Test', 'Android',  'Nexus10',    'MaliT604',    'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_10'}),
      ('Perf', 'Android',  'Nexus10',    'MaliT604',    'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'nexus_10'}),
      ('Test', 'Android',  'GalaxyNexus','SGX540',      'Arm7',   'Debug',   None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'galaxy_nexus'}),
      ('Test', 'Android',  'GalaxyNexus','SGX540',      'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'galaxy_nexus'}),
      ('Perf', 'Android',  'GalaxyNexus','SGX540',      'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'galaxy_nexus'}),
      ('Test', 'Android',  'Xoom',       'Tegra2',      'Arm7',   'Debug',   None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'xoom'}),
      ('Test', 'Android',  'Xoom',       'Tegra2',      'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'xoom'}),
      ('Perf', 'Android',  'Xoom',       'Tegra2',      'Arm7',   'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'xoom'}),
      ('Test', 'Android',  'IntelRhb',   'SGX544',      'x86',    'Debug',   None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'intel_rhb'}),
      ('Test', 'Android',  'IntelRhb',   'SGX544',      'x86',    'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'intel_rhb'}),
      ('Perf', 'Android',  'IntelRhb',   'SGX544',      'x86',    'Release', None,           None,      f_android, LINUX,  S_PERCOMMIT, {'device': 'intel_rhb'}),
  ]

  master_builders_cfg.setup_builders_from_config_list(
      builder_specs,
      helper,
      do_upload_render_results,
      do_upload_bench_results,
      master_builders_cfg.Builder)


def setup_all_builders(helper, do_upload_render_results,
                       do_upload_bench_results):
  """Set up all builders for the FYI master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
  """
  setup_test_and_perf_builders(helper, do_upload_render_results,
                               do_upload_bench_results)
