# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

from master import master_config
from skia_master_scripts import android_factory
from skia_master_scripts import factory as skia_factory
from skia_master_scripts import housekeeping_percommit_factory
from skia_master_scripts import housekeeping_periodic_factory
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

  # Linux (Ubuntu12) on Shuttle with ATI5770 graphics card
  defaults['category'] = 'Shuttle_Ubuntu12_ATI5770'
  B('Skia_Shuttle_Ubuntu12_ATI5770_Float_Debug', 'f_skia_shuttle_ubuntu12_ati5770_float_debug',
      scheduler='skia_rel')
  F('f_skia_shuttle_ubuntu12_ati5770_float_debug', skia_factory.SkiaFactory(
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Shuttle_Ubuntu12_ATI5770_Float_Debug',
      ).Build())
  B('Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_32', 'f_skia_shuttle_ubuntu12_ati5770_float_release_32',
      scheduler='skia_rel')
  F('f_skia_shuttle_ubuntu12_ati5770_float_release_32', skia_factory.SkiaFactory(
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0 skia_arch_width=32'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=perf_output_basedir_linux,
      builder_name='Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_32',
      ).Build())
  B('Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64', 'f_skia_shuttle_ubuntu12_ati5770_float_release_64',
      scheduler='skia_rel')
  F('f_skia_shuttle_ubuntu12_ati5770_float_release_64', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=0 skia_arch_width=64'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=perf_output_basedir_linux,
      builder_name='Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64',
      ).Build())

  # Android (runs on a Linux buildbot slave)...
  defaults['category'] = 'android'
  B('Skia_NexusS_4-1_Float_Debug', 'f_skia_nexus_s_4-1_float_debug',
      scheduler='skia_rel')
  F('f_skia_nexus_s_4-1_float_debug', android_factory.AndroidFactory(
      do_upload_results=False,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-android-nexus-s',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_NexusS_4-1_Float_Debug',
      ).Build(device='nexus_s'))
  B('Skia_NexusS_4-1_Float_Release', 'f_skia_nexus_s_4-1_float_release',
      scheduler='skia_rel')
  F('f_skia_nexus_s_4-1_float_release', android_factory.AndroidFactory(
      do_upload_results=do_upload_results,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-android-nexus-s',
      perf_output_basedir=perf_output_basedir_linux,
      builder_name='Skia_NexusS_4-1_Float_Release',
      ).Build(device='nexus_s'))
  B('Skia_Xoom_4-1_Float_Debug', 'f_skia_xoom_4-1_float_debug',
      scheduler='skia_rel')
  F('f_skia_xoom_4-1_float_debug', android_factory.AndroidFactory(
      do_upload_results=False,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-android-xoom',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Xoom_4-1_Float_Debug',
      ).Build(device='xoom'))
  B('Skia_Xoom_4-1_Float_Release', 'f_skia_xoom_4-1_float_release',
      scheduler='skia_rel')
  F('f_skia_xoom_4-1_float_release', android_factory.AndroidFactory(
      do_upload_results=do_upload_results,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-android-xoom',
      perf_output_basedir=perf_output_basedir_linux,
      builder_name='Skia_Xoom_4-1_Float_Release',
      ).Build(device='xoom'))
  B('Skia_GalaxyNexus_4-1_Float_Debug', 'f_skia_galaxy_nexus_4-1_float_debug',
      scheduler='skia_rel')
  F('f_skia_galaxy_nexus_4-1_float_debug', android_factory.AndroidFactory(
      do_upload_results=False,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-android-galaxy-nexus',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_GalaxyNexus_4-1_Float_Debug',
      ).Build(device='galaxy_nexus'))
  B('Skia_GalaxyNexus_4-1_Float_Release', 'f_skia_galaxy_nexus_4-1_float_release',
      scheduler='skia_rel')
  F('f_skia_galaxy_nexus_4-1_float_release', android_factory.AndroidFactory(
      do_upload_results=do_upload_results,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-android-galaxy-nexus',
      perf_output_basedir=perf_output_basedir_linux,
      builder_name='Skia_GalaxyNexus_4-1_Float_Release',
      ).Build(device='galaxy_nexus'))
  B('Skia_Nexus7_4-1_Float_Debug', 'f_skia_nexus7_4-1_float_debug',
      scheduler='skia_rel')
  F('f_skia_nexus7_4-1_float_debug', android_factory.AndroidFactory(
      do_upload_results=False,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-android-nexus-7',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Nexus7_4-1_Float_Debug',
      ).Build(device='nexus_7'))
  B('Skia_Nexus7_4-1_Float_Release', 'f_skia_nexus7_4-1_float_release',
      scheduler='skia_rel')
  F('f_skia_nexus7_4-1_float_release', android_factory.AndroidFactory(
      do_upload_results=do_upload_results,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-android-nexus-7',
      perf_output_basedir=perf_output_basedir_linux,
      builder_name='Skia_Nexus7_4-1_Float_Release',
      ).Build(device='nexus_7'))

  # Mac 10.6 (SnowLeopard) ...
  defaults['category'] = 'mac-10.6'
  B('Skia_Mac_Float_Debug', 'f_skia_mac_float_debug',
      scheduler='skia_rel')
  F('f_skia_mac_float_debug', skia_factory.SkiaFactory(
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1'},
      gm_image_subdir='base-macmini',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Mac_Float_Debug',
      ).Build())
  B('Skia_Mac_Float_NoDebug_32', 'f_skia_mac_float_nodebug_32',
      scheduler='skia_rel')
  F('f_skia_mac_float_nodebug_32', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1 skia_arch_width=32'},
      gm_image_subdir='base-macmini',
      perf_output_basedir=perf_output_basedir_mac,
      builder_name='Skia_Mac_Float_NoDebug_32',
      ).Build())
  B('Skia_Mac_Float_NoDebug_64', 'f_skia_mac_float_nodebug_64',
      scheduler='skia_rel')
  F('f_skia_mac_float_nodebug_64', skia_factory.SkiaFactory(
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1 skia_arch_width=64'},
      gm_image_subdir='base-macmini',
      perf_output_basedir=perf_output_basedir_mac,
      builder_name='Skia_Mac_Float_NoDebug_64',
      ).Build())

  # Mac 10.7 (Lion) ...
  defaults['category'] = 'mac-10.7'
  B('Skia_MacMiniLion_Float_Debug', 'f_skia_MacMiniLion_float_debug',
      scheduler='skia_rel')
  F('f_skia_MacMiniLion_float_debug', skia_factory.SkiaFactory(
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1'},
      gm_image_subdir='base-macmini-lion-float',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_MacMiniLion_Float_Debug',
      ).Build())
  B('Skia_MacMiniLion_Float_NoDebug_32', 'f_skia_MacMiniLion_float_nodebug_32',
      scheduler='skia_rel')
  F('f_skia_MacMiniLion_float_nodebug_32', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1 skia_arch_width=32'},
      gm_image_subdir='base-macmini-lion-float',
      perf_output_basedir=perf_output_basedir_mac,
      builder_name='Skia_MacMiniLion_Float_NoDebug_32',
      ).Build())
  B('Skia_MacMiniLion_Float_NoDebug_64', 'f_skia_MacMiniLion_float_nodebug_64',
      scheduler='skia_rel')
  F('f_skia_MacMiniLion_float_nodebug_64', skia_factory.SkiaFactory(
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1 skia_arch_width=64'},
      gm_image_subdir='base-macmini-lion-float',
      perf_output_basedir=perf_output_basedir_mac,
      builder_name='Skia_MacMiniLion_Float_NoDebug_64',
      ).Build())

  # Windows7 running on Shuttle PC with Intel Core i7-2600 with on-CPU graphics
  defaults['category'] = 'Shuttle_Win7_Intel'
  B('Skia_Shuttle_Win7_Intel_Float_Debug', 'f_skia_shuttle_win7_intel_float_debug',
      scheduler='skia_rel')
  F('f_skia_shuttle_win7_intel_float_debug', skia_factory.SkiaFactory(
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-shuttle-win7-intel-float',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Shuttle_Win7_Intel_Float_Debug',
      ).Build())
  B('Skia_Shuttle_Win7_Intel_Float_Release_32', 'f_skia_shuttle_win7_intel_float_release_32',
      scheduler='skia_rel')
  F('f_skia_shuttle_win7_intel_float_release_32', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_arch_width=32'},
      gm_image_subdir='base-shuttle-win7-intel-float',
      perf_output_basedir=perf_output_basedir_windows,
      builder_name='Skia_Shuttle_Win7_Intel_Float_Release_32',
      ).Build())
  B('Skia_Shuttle_Win7_Intel_Float_Release_64', 'f_skia_shuttle_win7_intel_float_release_64',
      scheduler='skia_rel')
  F('f_skia_shuttle_win7_intel_float_release_64', skia_factory.SkiaFactory(
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_arch_width=64'},
      gm_image_subdir='base-shuttle-win7-intel-float',
      perf_output_basedir=perf_output_basedir_windows,
      builder_name='Skia_Shuttle_Win7_Intel_Float_Release_64',
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
  B('Skia_Linux_NoGPU', 'f_skia_linux_no_gpu',
      scheduler='skia_rel')
  F('f_skia_linux_no_gpu', skia_factory.SkiaFactory(
      do_upload_results=False,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_gpu=0'},
      gm_image_subdir='base-shuttle_ubuntu12_ati5770',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Linux_NoGPU',
      ).Build())

  return helper.Update(c)
