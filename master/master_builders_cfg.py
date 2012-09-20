# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

from master import master_config
from skia_master_scripts import factory as skia_factory
from skia_master_scripts import housekeeping_percommit_factory
from skia_master_scripts import housekeeping_periodic_factory
from skia_master_scripts import utils
from skia_master_scripts.utils import MakeBuilderSet, MakeAndroidBuilderSet

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

  # Linux (Ubuntu12) on Shuttle with ATI5770 graphics card
  defaults['category'] = 'Shuttle_Ubuntu12_ATI5770'
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Shuttle_Ubuntu12_ATI5770_Float_%s_64',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_mesa=0 skia_arch_width=64'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=perf_output_basedir_linux)
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Shuttle_Ubuntu12_ATI5770_Float_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_mesa=0 skia_arch_width=32'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=perf_output_basedir_linux)

  # Android (runs on a Linux buildbot slave)...
  defaults['category'] = 'android'
  MakeAndroidBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_NexusS_4-1_Float_%s_32',
      device='nexus_s',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-nexus-s',
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Xoom_4-1_Float_%s_32',
      device='xoom',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-xoom',
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_GalaxyNexus_4-1_Float_%s_32',
      device='galaxy_nexus',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-galaxy-nexus',
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Nexus7_4-1_Float_%s_32',
      device='nexus_7',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-nexus-7',
      perf_output_basedir=perf_output_basedir_linux)

  # Mac 10.6 (SnowLeopard) ...
  defaults['category'] = 'mac-10.6'
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Mac_Float_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_mesa=1 skia_arch_width=32'},
      gm_image_subdir='base-macmini',
      perf_output_basedir=perf_output_basedir_mac)
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Mac_Float_%s_64',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_mesa=1 skia_arch_width=64'},
      gm_image_subdir='base-macmini',
      perf_output_basedir=perf_output_basedir_mac)

  # Mac 10.7 (Lion) ...
  defaults['category'] = 'mac-10.7'
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_MacMiniLion_Float_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_mesa=1 skia_arch_width=32'},
      gm_image_subdir='base-macmini-lion-float',
      perf_output_basedir=perf_output_basedir_mac)
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_MacMiniLion_Float_%s_64',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_mesa=1 skia_arch_width=64'},
      gm_image_subdir='base-macmini-lion-float',
      perf_output_basedir=perf_output_basedir_mac)

  # Windows7 running on Shuttle PC with Intel Core i7-2600 with on-CPU graphics
  defaults['category'] = 'Shuttle_Win7_Intel'
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Shuttle_Win7_Intel_Float_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_arch_width=32'},
      gm_image_subdir='base-shuttle-win7-intel-float',
      perf_output_basedir=perf_output_basedir_windows)
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Shuttle_Win7_Intel_Float_%s_64',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_arch_width=64'},
      gm_image_subdir='base-shuttle-win7-intel-float',
      perf_output_basedir=perf_output_basedir_windows)
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Shuttle_Win7_Intel_Float_ANGLE_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_angle=1 skia_arch_width=32'},
      gm_image_subdir='base-shuttle-win7-intel-angle',
      perf_output_basedir=perf_output_basedir_windows,
      gm_args=['--config', 'angle'],
      bench_args=['--config', 'ANGLE'])
  MakeBuilderSet(
      helper=helper,
      scheduler='skia_rel',
      builder_base_name='Skia_Shuttle_Win7_Intel_Float_DirectWrite_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      environment_variables=
          {'GYP_DEFINES':
           'skia_scalar=float skia_directwrite=1 skia_arch_width=32'},
      gm_image_subdir='base-shuttle-win7-intel-directwrite',
      perf_output_basedir=perf_output_basedir_windows)

  # House Keeping
  defaults['category'] = ' housekeeping'
  B('Skia_PerCommit_House_Keeping', 'f_skia_percommit_house_keeping',
      scheduler='skia_rel')
  F('f_skia_percommit_house_keeping',
      housekeeping_percommit_factory.HouseKeepingPerCommitFactory(
        do_upload_results=do_upload_results,
        target_platform=skia_factory.TARGET_PLATFORM_LINUX,
        builder_name='Skia_PerCommit_House_Keeping',
      ).Build())
  B('Skia_Periodic_House_Keeping', 'f_skia_periodic_house_keeping',
      scheduler='skia_periodic')
  F('f_skia_periodic_house_keeping',
      housekeeping_periodic_factory.HouseKeepingPeriodicFactory(
        do_upload_results=do_upload_results,
        target_platform=skia_factory.TARGET_PLATFORM_LINUX,
        builder_name='Skia_Periodic_House_Keeping',
      ).Build())

  # "Special" bots, running on Linux
  defaults['category'] = 'Linux-Special'
  B('Skia_Linux_NoGPU', 'f_skia_linux_no_gpu',
      scheduler='skia_rel')
  F('f_skia_linux_no_gpu', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration=skia_factory.CONFIG_DEBUG,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_gpu=0 skia_arch_width=64'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Linux_NoGPU',
      ).Build())

  return helper.Update(c)
