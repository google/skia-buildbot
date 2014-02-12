# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the FYI buildbot master to run.


from master_builders_cfg import f_deps, f_deps_results, f_percommit, f_periodic
from master_builders_cfg import f_skps, f_xsan, HousekeepingBuilder, LINUX
from skia_master_scripts import utils
from skia_master_scripts.moz2d_canary_factory \
    import Moz2DCanaryFactory as f_moz2d

import builder_name_schema
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
  #    Project,  OS,        Compiler, Arch,     Configuration, Flavor,  Workdir, GYP_DEFINES, Factory, Platform, Extra Args
  #
  builder_specs = [
      ('Moz2D', 'Ubuntu12', 'GCC',    'x86_64', 'Release',     None,    'skia',  None,        f_moz2d, LINUX,    {})
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
  #    Role,   OS,         Model,         GPU,      Arch,     Config,    Extra Config,GYP_DEFS,Factory,Target, Extra Args
  #
  builder_specs = [
      ('Test', 'Ubuntu13', 'ShuttleA',   'HD2000',  'x86_64', 'Debug',   'TSAN',      None,    f_xsan, LINUX,  {'sanitizer': 'thread'}),
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
  #   Frequency,    Scheduler,        Extra Config,       Factory,        Target, Extra Args
  #
  housekeepers = [
      ('PerCommit', 'skia_rel',       None,               f_percommit,    LINUX,  {}),
      ('Nightly',   'skia_nightly',   None,               f_periodic,     LINUX,  {}),
      ('Nightly',   'skia_nightly',  'DEPSRoll',          f_deps,         LINUX,  {}),
      ('Daily',     'skia_morning',    'DEPSRollResults',   f_deps_results, LINUX,  {'deps_roll_builder': 'Housekeeper-Nightly-DEPSRoll'}),
      ('Nightly',   'skia_evening', 'RecreateSKPs',      f_skps,         LINUX,  {}),
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
