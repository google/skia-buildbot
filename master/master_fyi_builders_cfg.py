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
from skia_master_scripts.recreate_skps_factory import \
    RecreateSKPsFactory as f_skps

import master_builders_cfg


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
