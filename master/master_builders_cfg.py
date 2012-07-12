# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

from master import master_config
from skia_master_scripts import android_factory
from skia_master_scripts import factory as skia_factory
from skia_master_scripts import housekeeping_factory

# Directory where we want to record performance data
#
# TODO(epoger): consider changing to reuse existing config.Master.perf_base_url,
# config.Master.perf_report_url_suffix, etc.
perf_output_basedir_linux = '../../../../perfdata'
perf_output_basedir_mac = perf_output_basedir_linux
perf_output_basedir_windows = '..\\..\\..\\..\\perfdata'

defaults = {}

def Update(config, active_master, c):
  helper = master_config.Helper(defaults)
  B = helper.Builder
  F = helper.Factory
  S = helper.Scheduler

  #
  # Main Scheduler for Skia
  #
  S('skia_rel', branch='trunk', treeStableTimer=60)

  #
  # Set up all the builders.
  #
  # Don't put spaces or 'funny characters' within the builder names, so that
  # we can safely use the builder name as part of a filepath.
  #
  do_upload_results = active_master.is_production_host

  # Linux...
  defaults['category'] = 'linux'
  B('Skia_Linux_Float_Debug', 'f_skia_linux_float_debug',
      scheduler='skia_rel')
  F('f_skia_linux_float_debug', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1'},
      gm_image_subdir='base-linux',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Linux_Float_Debug',
      ).Build())
  B('Skia_Linux_Float_NoDebug', 'f_skia_linux_float_nodebug',
      scheduler='skia_rel')
  F('f_skia_linux_float_nodebug', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1'},
      gm_image_subdir='base-linux',
      perf_output_basedir=perf_output_basedir_linux,
      builder_name='Skia_Linux_Float_NoDebug',
      ).Build())

  # Android (runs on a Linux buildbot slave)...
  defaults['category'] = 'android'
  B('Skia_NexusS_4-1_Float_Debug', 'f_skia_nexus_s_4-1_float_debug',
      scheduler='skia_rel')
  F('f_skia_nexus_s_4-1_float_debug', android_factory.AndroidFactory(
      do_upload_results=do_upload_results,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
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
      builder_name='Skia_NexusS_4-1_Float_Release',
      ).Build(device='nexus_s'))
  B('Skia_Xoom_4-1_Float_Debug', 'f_skia_xoom_4-1_float_debug',
      scheduler='skia_rel')
  F('f_skia_xoom_4-1_float_debug', android_factory.AndroidFactory(
      do_upload_results=do_upload_results,
      other_subdirs=['android'],
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
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
      builder_name='Skia_Xoom_4-1_Float_Release',
      ).Build(device='xoom'))

  # Mac 10.6 (SnowLeopard) ...
  defaults['category'] = 'mac-10.6'
  B('Skia_Mac_Float_Debug', 'f_skia_mac_float_debug',
      scheduler='skia_rel')
  F('f_skia_mac_float_debug', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1'},
      gm_image_subdir='base-macmini',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Mac_Float_Debug',
      ).Build())
  B('Skia_Mac_Float_NoDebug', 'f_skia_mac_float_nodebug',
      scheduler='skia_rel')
  F('f_skia_mac_float_nodebug', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1'},
      gm_image_subdir='base-macmini',
      perf_output_basedir=perf_output_basedir_mac,
      builder_name='Skia_Mac_Float_NoDebug',
      ).Build())

  # Mac 10.7 (Lion) ...
  defaults['category'] = 'mac-10.7'
  B('Skia_MacMiniLion_Float_Debug', 'f_skia_MacMiniLion_float_debug',
      scheduler='skia_rel')
  F('f_skia_MacMiniLion_float_debug', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1'},
      gm_image_subdir='base-macmini-lion-float',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_MacMiniLion_Float_Debug',
      ).Build())
  B('Skia_MacMiniLion_Float_NoDebug', 'f_skia_MacMiniLion_float_nodebug',
      scheduler='skia_rel')
  F('f_skia_MacMiniLion_float_nodebug', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_MAC,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float skia_mesa=1'},
      gm_image_subdir='base-macmini-lion-float',
      perf_output_basedir=perf_output_basedir_mac,
      builder_name='Skia_MacMiniLion_Float_NoDebug',
      ).Build())

  # Original Windows bot...
  defaults['category'] = 'windows'
  B('Skia_Win32_Float_Debug', 'f_skia_win32_float_debug',
      scheduler='skia_rel')
  F('f_skia_win32_float_debug', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-win',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Win32_Float_Debug',
      ).Build())
  B('Skia_Win32_Float_NoDebug', 'f_skia_win32_float_nodebug',
      scheduler='skia_rel')
  F('f_skia_win32_float_nodebug', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-win',
      perf_output_basedir=perf_output_basedir_windows,
      builder_name='Skia_Win32_Float_NoDebug',
      ).Build())

  # Windows7 running on Shuttle PC with Intel Core i7-2600 with on-CPU graphics
  defaults['category'] = 'Shuttle_Win7_Intel'
  B('Skia_Shuttle_Win7_Intel_Float_Debug', 'f_skia_shuttle_win7_intel_float_debug',
      scheduler='skia_rel')
  F('f_skia_shuttle_win7_intel_float_debug', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      configuration='Debug',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-shuttle-win7-intel-float',
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name='Skia_Shuttle_Win7_Intel_Float_Debug',
      ).Build())
  B('Skia_Shuttle_Win7_Intel_Float_Release', 'f_skia_shuttle_win7_intel_float_release',
      scheduler='skia_rel')
  F('f_skia_shuttle_win7_intel_float_release', skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_WIN32,
      configuration='Release',
      environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
      gm_image_subdir='base-shuttle-win7-intel-float',
      perf_output_basedir=perf_output_basedir_windows,
      builder_name='Skia_Shuttle_Win7_Intel_Float_Release',
      ).Build())

  # House Keeping
  defaults['category'] = ' housekeeping'
  B('Skia_House_Keeping', 'f_skia_house_keeping',
      scheduler='skia_rel')
  F('f_skia_house_keeping', housekeeping_factory.HouseKeepingFactory(
      do_upload_results=do_upload_results,
      target_platform=skia_factory.TARGET_PLATFORM_LINUX,
      configuration='HouseKeeping',
      perf_output_basedir=None, # no perf measurement for housekeeping builds
      builder_name='Skia_House_Keeping',
      ).Build())

  return helper.Update(c)
