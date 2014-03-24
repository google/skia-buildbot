# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the private buildbot master to run.


import master_builders_cfg
from master_builders_cfg import f_android, LINUX, S_PERCOMMIT


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
  #    Role,   OS,         Model,   GPU,      Arch,   Config,    Extra Config,GYP_DEFS, Factory,   Target, Scheduler,   Extra Args
  #
  builder_specs = [
      ('Test', 'Android',  'Logan', 'Nvidia', 'Arm7', 'Debug',   None,        None,     f_android, LINUX,  S_PERCOMMIT, {'device': 'nvidia_logan'}),
      ('Test', 'Android',  'Logan', 'Nvidia', 'Arm7', 'Release', None,        None,     f_android, LINUX,  S_PERCOMMIT, {'device': 'nvidia_logan'}),
      ('Perf', 'Android',  'Logan', 'Nvidia', 'Arm7', 'Release', None,        None,     f_android, LINUX,  S_PERCOMMIT, {'device': 'nvidia_logan'}),
  ]

  master_builders_cfg.setup_builders_from_config_list(
      builder_specs,
      helper,
      do_upload_render_results,
      do_upload_bench_results,
      master_builders_cfg.Builder)


def setup_all_builders(helper, do_upload_render_results,
                       do_upload_bench_results):
  """Set up all builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
  """
  setup_test_and_perf_builders(helper, do_upload_render_results,
                               do_upload_bench_results)

