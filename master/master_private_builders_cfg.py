# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the private buildbot master to run.


import master_builders_cfg
from master_builders_cfg import GYP_NVPR, f_android, LINUX


def setup_compile_builders(helper, do_upload_results):
  """Set up all compile builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their
          results.
  """
  #
  #                            COMPILE BUILDERS
  #
  #    OS          Compiler  Config     Arch     Extra Config    GYP_DEFS   WERR  Factory Args
  #
  builder_specs = [
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',  'NvidiaLogan',  GYP_NVPR,  True, {'device': 'nvidia_logan'}, f_android, LINUX),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',  'NvidiaLogan',  GYP_NVPR,  True, {'device': 'nvidia_logan'}, f_android, LINUX),
  ]

  master_builders_cfg.setup_compile_builders_from_config_list(
      builder_specs, helper, do_upload_results)


def setup_test_and_perf_builders(helper, do_upload_results):
  """Set up all Test and Perf builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  #
  #                            TEST AND PERF BUILDERS
  #
  #    Role    OS          Model          GPU       Arch    Config     Extra Config GYP_DEFS GM Subdir Factory Args
  #
  builder_specs = [
      ('Test', 'Android',  'Logan', 'Nvidia', 'Arm7', 'Debug',   None,        GYP_NVPR, 'base-android-logan', {'device': 'nvidia_logan'}, f_android, LINUX),
      ('Test', 'Android',  'Logan', 'Nvidia', 'Arm7', 'Release', None,        GYP_NVPR, 'base-android-logan', {'device': 'nvidia_logan'}, f_android, LINUX),
      ('Perf', 'Android',  'Logan', 'Nvidia', 'Arm7', 'Release', None,        GYP_NVPR, None, {'device': 'nvidia_logan'}, f_android, LINUX),
  ]

  master_builders_cfg.setup_test_and_perf_builders_from_config_list(
      builder_specs, helper, do_upload_results)


def setup_all_builders(helper, do_upload_results):
  """Set up all builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  setup_compile_builders(helper, do_upload_results)
  setup_test_and_perf_builders(helper, do_upload_results)

