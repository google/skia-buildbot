# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the private buildbot master to run.


from skia_master_scripts import android_factory
from skia_master_scripts import factory as skia_factory

import master_builders_cfg
from master_builders_cfg import GYP_NVPR


def setup_compile_builders(helper, do_upload_results):
  """Set up all compile builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their
          results.
  """
  # builder_specs is a list whose entries describe compile builders.
  builder_specs = []
  #
  #                            COMPILE BUILDERS
  #
  #    OS          Compiler  Config     Arch     Extra Config    GYP_DEFS   WERR  Factory Args
  #
  f = android_factory.AndroidFactory
  p = skia_factory.TARGET_PLATFORM_LINUX
  builder_specs.extend([
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',  'NvidiaLogan',  GYP_NVPR,  True, {'device': 'nvidia_logan'}, f, p),
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',  'NvidiaLogan',  GYP_NVPR,  True, {'device': 'nvidia_logan'}, f, p),
  ])

  master_builders_cfg.setup_compile_builders_from_config_list(
      builder_specs, helper, do_upload_results)


def setup_test_and_perf_builders(helper, do_upload_results):
  """Set up all Test and Perf builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  # builder_specs is a list whose entries describe Test and Perf builders.
  builder_specs = []
  #
  #                            TEST AND PERF BUILDERS
  #
  #    Role    OS          Model          GPU       Arch    Config     Extra Config GYP_DEFS GM Subdir Factory Args
  #
  f = android_factory.AndroidFactory
  p = skia_factory.TARGET_PLATFORM_LINUX
  builder_specs.extend([
      ('Test', 'Android',  'Logan', 'Nvidia', 'Arm7', 'Debug',   None,        GYP_NVPR, 'base-android-logan', {'device': 'nvidia_logan'}, f, p),
      ('Test', 'Android',  'Logan', 'Nvidia', 'Arm7', 'Release', None,        GYP_NVPR, 'base-android-logan', {'device': 'nvidia_logan'}, f, p),
      ('Perf', 'Android',  'Logan', 'Nvidia', 'Arm7', 'Release', None,        GYP_NVPR, None, {'device': 'nvidia_logan'}, f, p),
  ])

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

