# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

from skia_master_scripts import utils

# Directory where we want to record performance data
#
# TODO(epoger): consider changing to reuse existing config.Master.perf_base_url,
# config.Master.perf_report_url_suffix, etc.
perf_output_basedir_linux = '../../../../perfdata'
perf_output_basedir_mac = perf_output_basedir_linux
perf_output_basedir_windows = '..\\..\\..\\..\\perfdata'

defaults = {}

def Update(config, active_master, c):
  helper = utils.SkiaHelper(defaults)
  B = helper.Builder
  F = helper.Factory

  #
  # Main (per-commit) Scheduler for Skia
  #
  helper.AnyBranchScheduler('skia_rel', branches=utils.SKIA_PRIMARY_SUBDIRS)

  #
  # Periodic Scheduler for Skia. The timezone the PeriodicScheduler is run in is
  # the timezone of the buildbot master. Currently this is EST because it runs
  # in Atlanta.
  #
  helper.PeriodicScheduler('skia_periodic', branch='trunk', minute=0, hour=2)

  #
  # Set up all the builders.
  #
  # Don't put spaces or 'funny characters' within the builder names, so that
  # we can safely use the builder name as part of a filepath.
  #
  do_upload_results = active_master.is_production_host

  ########## LIST ALL PRIVATELY VISIBLE BUILDERS HERE ##########
  pass

  return helper.Update(c)

