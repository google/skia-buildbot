# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the FYI buildbot master to run.


#pylint: disable=C0301


from master_builders_cfg import HousekeepingBuilder, LINUX
from master_builders_cfg import S_PERCOMMIT, S_NIGHTLY, S_15MINS

from skia_master_scripts.auto_roll_factory import AutoRollFactory as f_autoroll
from skia_master_scripts.housekeeping_monitoring_factory import \
    HouseKeepingMonitoringFactory as f_monitor
from skia_master_scripts.housekeeping_percommit_factory import \
    HouseKeepingPerCommitFactory as f_percommit
from skia_master_scripts.housekeeping_periodic_factory import \
    HouseKeepingPeriodicFactory as f_periodic
from skia_master_scripts.moz2d_canary_factory import \
    Moz2DCanaryFactory as f_moz2d
from skia_master_scripts.recreate_skps_factory import \
    RecreateSKPsFactory as f_skps

import master_builders_cfg


# GYP_DEFINES.
MOZ2D = repr({'skia_moz2d': '1'})


def setup_canaries(helper, do_upload_render_results, do_upload_bench_results):
  """Set up the Canary builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
  """
  #
  #                          CANARY BUILDERS
  #
  #    Project,  OS,        Compiler, Arch,     Configuration, Flavor,  Workdir, GYP_DEFINES, Factory, Platform, Scheduler, Extra Args
  #
  builder_specs = [
      ('Moz2D', 'Ubuntu12', 'GCC',    'x86_64', 'Release',     None,    'skia',  MOZ2D,       f_moz2d, LINUX,    S_PERCOMMIT, {})
  ]

  master_builders_cfg.setup_builders_from_config_list(
      builder_specs,
      helper,
      do_upload_render_results,
      do_upload_bench_results,
      master_builders_cfg.CanaryBuilder)


def setup_housekeepers(helper, do_upload_render_results,
                       do_upload_bench_results):
  """Set up the Housekeeping builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_render_results: bool; whether the builders should upload their
          render results.
      do_upload_bench_results: bool; whether the builders should upload their
          bench results.
  """
  #
  #                          HOUSEKEEPING BUILDERS
  #
  #   Frequency,    Extra Config,       Factory,        Target, Scheduler,   Extra Args
  #
  housekeepers = [
      ('PerCommit', None,               f_percommit,    LINUX,  S_PERCOMMIT, {}),
      ('PerCommit', 'AutoRoll',         f_autoroll,     LINUX,  S_15MINS,    {}),
      ('Nightly',   None,               f_periodic,     LINUX,  S_NIGHTLY,   {}),
      ('Nightly',   'Monitoring',       f_monitor,      LINUX,  S_NIGHTLY,   {}),
      ('Nightly',   'RecreateSKPs',     f_skps,         LINUX,  S_NIGHTLY,   {}),
  ]

  master_builders_cfg.setup_builders_from_config_list(housekeepers, helper,
                                                      do_upload_render_results,
                                                      do_upload_bench_results,
                                                      HousekeepingBuilder)


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
  setup_canaries(helper, do_upload_render_results, do_upload_bench_results)
  setup_housekeepers(helper=helper,
                     do_upload_render_results=do_upload_render_results,
                     do_upload_bench_results=do_upload_bench_results)
