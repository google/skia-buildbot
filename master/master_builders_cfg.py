# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

from master import master_config
from skia_master_scripts import chromeos_factory
from skia_master_scripts import factory as skia_factory
from skia_master_scripts import ios_factory
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
  # Default (per-commit) Scheduler for Skia. Only use this for builders which
  # do not care about commits outside of SKIA_PRIMARY_SUBDIRS.
  #
  helper.AnyBranchScheduler('skia_rel', branches=utils.SKIA_PRIMARY_SUBDIRS)

  #
  # Periodic Scheduler for Skia. The timezone the PeriodicScheduler is run in is
  # the timezone of the buildbot master. Currently this is EST because it runs
  # in Atlanta.
  #
  helper.PeriodicScheduler('skia_periodic', branch='trunk', minute=0, hour=2)

  # Scheduler for Skia trybots.
  helper.TryScheduler('skia_try')

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
      builder_base_name='Skia_Shuttle_Ubuntu12_ATI5770_Float_%s_64',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_mesa=0 skia_arch_width=64'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=perf_output_basedir_linux)
  MakeBuilderSet(
      helper=helper,
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
      builder_base_name='Skia_NexusS_4-1_Float_%s_32',
      device='nexus_s',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-nexus-s',
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      builder_base_name='Skia_Xoom_4-1_Float_%s_32',
      device='xoom',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-xoom',
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      builder_base_name='Skia_GalaxyNexus_4-1_Float_%s_32',
      device='galaxy_nexus',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-galaxy-nexus',
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      builder_base_name='Skia_Nexus4_4-1_Float_%s_32',
      device='nexus_4',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-nexus-4',
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      builder_base_name='Skia_Nexus7_4-1_Float_%s_32',
      device='nexus_7',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-nexus-7',
      perf_output_basedir=perf_output_basedir_linux)
  MakeAndroidBuilderSet(
      helper=helper,
      builder_base_name='Skia_Nexus10_4-1_Float_%s_32',
      device='nexus_10',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-android-nexus-10',
      perf_output_basedir=perf_output_basedir_linux)

  # Mac 10.6 (SnowLeopard) ...
  defaults['category'] = 'mac-10.6'
  MakeBuilderSet(
      helper=helper,
      builder_base_name='Skia_Mac_Float_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      environment_variables=
          {'GYP_DEFINES': 'skia_osx_sdkroot=macosx10.6 skia_scalar=float skia_mesa=1 skia_arch_width=32'},
      gm_image_subdir='base-macmini',
      perf_output_basedir=perf_output_basedir_mac)
  MakeBuilderSet(
      helper=helper,
      builder_base_name='Skia_Mac_Float_%s_64',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      environment_variables=
          {'GYP_DEFINES': 'skia_osx_sdkroot=macosx10.6 skia_scalar=float skia_mesa=1 skia_arch_width=64'},
      gm_image_subdir='base-macmini',
      perf_output_basedir=perf_output_basedir_mac)

  # Mac 10.7 (Lion) ...
  defaults['category'] = 'mac-10.7'
  MakeBuilderSet(
      helper=helper,
      builder_base_name='Skia_MacMiniLion_Float_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_mesa=1 skia_arch_width=32'},
      gm_image_subdir='base-macmini-lion-float',
      perf_output_basedir=perf_output_basedir_mac)
  MakeBuilderSet(
      helper=helper,
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
      builder_base_name='Skia_Shuttle_Win7_Intel_Float_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_arch_width=32'},
      gm_image_subdir='base-shuttle-win7-intel-float',
      perf_output_basedir=perf_output_basedir_windows)
  MakeBuilderSet(
      helper=helper,
      builder_base_name='Skia_Shuttle_Win7_Intel_Float_%s_64',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_arch_width=64'},
      gm_image_subdir='base-shuttle-win7-intel-float',
      perf_output_basedir=perf_output_basedir_windows)

  # Special-purpose Win7 builders
  defaults['category'] = 'Win7-Special'
  MakeBuilderSet(
      helper=helper,
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
      builder_base_name='Skia_Shuttle_Win7_Intel_Float_DirectWrite_%s_32',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      environment_variables=
          {'GYP_DEFINES':
           'skia_scalar=float skia_directwrite=1 skia_arch_width=32'},
      gm_image_subdir='base-shuttle-win7-intel-directwrite',
      perf_output_basedir=perf_output_basedir_windows)

  defaults['category'] = 'iOS'
  B('Skia_iOS_32', 'f_skia_ios_32', scheduler='skia_rel')
  F('f_skia_ios_32', ios_factory.iOSFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration=skia_factory.CONFIG_DEBUG,
      environment_variables={'GYP_DEFINES': 'skia_os=ios'},
      gm_image_subdir=None,
      perf_output_basedir=None,
      builder_name='Skia_iOS_32',
      ).Build())

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

  # Use the Ubuntu12_ATI5770 scheduler created by MakeBuilderSet so that this
  # builder will be triggered by commits inside the
  # 'gm-expected/base-shuttle_ubuntu12_ati5770' directory. This is a bit of a
  # hack, since we shouldn't know the names of the schedulers created inside
  # MakeBuilderSet, but it lets us avoid creating a redundant scheduler here.
  B('Skia_Linux_NoGPU', 'f_skia_linux_no_gpu',
    scheduler='Skia_Shuttle_Ubuntu12_ATI5770_Float_Scheduler_64')
  F('f_skia_linux_no_gpu', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration=skia_factory.CONFIG_DEBUG,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_gpu=0 skia_arch_width=64'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=None, # no perf measurement for debug builds
      bench_pictures_cfg='no_gpu',
      builder_name='Skia_Linux_NoGPU',
      ).Build())

  B('Skia_Linux_NoGPU_Trybot', 'f_skia_linux_no_gpu_trybot',
    scheduler='skia_try')
  F('f_skia_linux_no_gpu_trybot', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      do_patch_step=True,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration=skia_factory.CONFIG_DEBUG,
      environment_variables=
          {'GYP_DEFINES': 'skia_scalar=float skia_gpu=0 skia_arch_width=64'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Linux_NoGPU_Trybot',
      ).Build())

  defaults['category'] = 'ChromeOS'
  B('Skia_ChromeOS_Alex_Debug_32', 'f_skia_chromeos_alex_debug_32',
      scheduler='skia_rel')
  F('f_skia_chromeos_alex_debug_32', chromeos_factory.ChromeOSFactory(
      ssh_host='192.168.1.134',
      ssh_port='22',
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration=skia_factory.CONFIG_DEBUG,
      environment_variables=
          {'GYP_DEFINES': 'skia_arch_width=32 skia_gpu=0'},
      gm_image_subdir=None,
      perf_output_basedir=None, # no perf measurement for debug builds
      bench_pictures_cfg='no_gpu',
      builder_name='Skia_ChromeOS_Alex_Debug_32',
      ).Build())

  return helper.Update(c)
