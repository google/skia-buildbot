# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

from skia_master_scripts import utils
from skia_master_scripts import android_factory
from skia_master_scripts import factory as skia_factory
from skia_master_scripts.utils import MakeAndroidBuilderSet
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
  MakeAndroidBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Private_Builder_%s_001',
      device='nexus_s',
      serial='5D327F9B4103E10F',
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir=None,
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Private_Builder_%s_002',
      device='nexus_s',
      serial='0012746f51cea6b9',
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir=None,
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Private_Builder_%s_003',
      device='nexus_s',
      serial='B6E7F6341038B13',
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir=None,
      perf_output_basedir=perf_output_basedir_linux)
  return helper.Update(c)

