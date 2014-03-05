# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the FYI buildbot master to run.


from master_builders_cfg import f_deps, f_deps_results, f_percommit, f_periodic
from master_builders_cfg import f_skps, f_xsan, HousekeepingBuilder, LINUX

# Schedulers
from master_builders_cfg import S_PERCOMMIT, S_NIGHTLY, S_EVENING, S_MORNING
from master_builders_cfg import S_RECREATE_SKPS

from skia_master_scripts.moz2d_canary_factory \
    import Moz2DCanaryFactory as f_moz2d

import master_builders_cfg


def setup_canaries(helper, do_upload_results):
  """Set up the Canary builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  #
  #                          CANARY BUILDERS
  #
  #    Project,  OS,        Compiler, Arch,     Configuration, Flavor,  Workdir, GYP_DEFINES, Factory, Platform, Scheduler, Extra Args
  #
  builder_specs = [
      ('Moz2D', 'Ubuntu12', 'GCC',    'x86_64', 'Release',     None,    'skia',  None,        f_moz2d, LINUX,    S_PERCOMMIT, {})
  ]

  master_builders_cfg.setup_builders_from_config_list(
      builder_specs, helper, do_upload_results,
      master_builders_cfg.CanaryBuilder)


def setup_test_and_perf_builders(helper, do_upload_results):
  """Set up all Test and Perf builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  #
  #                            TEST AND PERF BUILDERS
  #
  #    Role,   OS,         Model,         GPU,      Arch,     Config,    Extra Config,GYP_DEFS,Factory,Target, Scheduler, Extra Args
  #
  builder_specs = [
      ('Test', 'Ubuntu13', 'ShuttleA',   'HD2000',  'x86_64', 'Debug',   'TSAN',      None,    f_xsan, LINUX,  S_PERCOMMIT, {'sanitizer': 'thread'}),
  ]

  master_builders_cfg.setup_builders_from_config_list(
      builder_specs, helper, do_upload_results, master_builders_cfg.Builder)


def setup_housekeepers(helper, do_upload_results):
  """Set up the Housekeeping builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  #
  #                          HOUSEKEEPING BUILDERS
  #
  #   Frequency,    Extra Config,       Factory,        Target, Scheduler,       Extra Args
  #
  housekeepers = [
      ('PerCommit', None,               f_percommit,    LINUX,  S_PERCOMMIT,     {}),
      ('Nightly',   None,               f_periodic,     LINUX,  S_NIGHTLY,       {}),
      ('Nightly',   'DEPSRoll',         f_deps,         LINUX,  S_EVENING,       {}),
      ('Daily',     'DEPSRollResults',  f_deps_results, LINUX,  S_MORNING,       {'deps_roll_builder': 'Housekeeper-Nightly-DEPSRoll'}),
      ('Nightly',   'RecreateSKPs',     f_skps,         LINUX,  S_RECREATE_SKPS, {}),
  ]

  master_builders_cfg.setup_builders_from_config_list(housekeepers, helper,
                                                      do_upload_results,
                                                      HousekeepingBuilder)


def setup_all_builders(helper, do_upload_results):
  """Set up all builders for the FYI master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  setup_test_and_perf_builders(helper, do_upload_results)
  setup_canaries(helper, do_upload_results)
  setup_housekeepers(helper=helper, do_upload_results=do_upload_results)
