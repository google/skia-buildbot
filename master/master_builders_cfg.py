# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

import os

from master import master_config

# import from local scripts/ directory
import skia_factory

defaults = {}

helper = master_config.Helper(defaults)
B = helper.Builder
D = helper.Dependent
F = helper.Factory
S = helper.Scheduler

defaults['category'] = 'linux'


# Directory where we want to record performance data
#
# TODO(epoger): consider changing to reuse existing config.Master.perf_base_url,
# config.Master.perf_report_url_suffix, etc.
perf_output_basedir_linux = '../../../../perfdata'
perf_output_basedir_mac = perf_output_basedir_linux
perf_output_basedir_windows = '..\\..\\..\\..\\perfdata'


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
# TODO(epoger): for now, I am manually adding the builder name to each
# perf_output_dir below. This should be automatic, because when we generate
# the graphs we require that they were assembled with builder name.
#

# Linux...
B('Skia_Linux_Fixed_Debug', 'f_skia_linux_fixed_debug',
  scheduler='skia_rel')
F('f_skia_linux_fixed_debug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_LINUX,
    configuration='Debug',
    environment_variables={'GYP_DEFINES': 'skia_scalar=fixed'},
    gm_image_subdir='base-linux-fixed',
    perf_output_dir=None, # no perf measurement for debug builds
    ).Build())
B('Skia_Linux_Fixed_NoDebug', 'f_skia_linux_fixed_nodebug',
  scheduler='skia_rel')
F('f_skia_linux_fixed_nodebug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_LINUX,
    configuration='Release',
    environment_variables={'GYP_DEFINES': 'skia_scalar=fixed'},
    gm_image_subdir='base-linux-fixed',
    perf_output_dir=os.path.join(
        perf_output_basedir_linux, 'Skia_Linux_Fixed_NoDebug'),
    ).Build())
B('Skia_Linux_Float_Debug', 'f_skia_linux_float_debug',
  scheduler='skia_rel')
F('f_skia_linux_float_debug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_LINUX,
    configuration='Debug',
    environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
    gm_image_subdir='base-linux',
    perf_output_dir=None, # no perf measurement for debug builds
    ).Build())
B('Skia_Linux_Float_NoDebug', 'f_skia_linux_float_nodebug',
  scheduler='skia_rel')
F('f_skia_linux_float_nodebug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_LINUX,
    configuration='Release',
    environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
    gm_image_subdir='base-linux',
    perf_output_dir=os.path.join(
        perf_output_basedir_linux, 'Skia_Linux_Float_NoDebug'),
    ).Build())

# Mac...
B('Skia_Mac_Fixed_Debug', 'f_skia_mac_fixed_debug',
  scheduler='skia_rel')
F('f_skia_mac_fixed_debug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_MAC,
    configuration='Debug',
    environment_variables={'GYP_DEFINES': 'skia_scalar=fixed'},
    gm_image_subdir='base-MacPro-fixed',
    perf_output_dir=None, # no perf measurement for debug builds
    ).Build())
B('Skia_Mac_Fixed_NoDebug', 'f_skia_mac_fixed_nodebug',
  scheduler='skia_rel')
F('f_skia_mac_fixed_nodebug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_MAC,
    configuration='Release',
    environment_variables={'GYP_DEFINES': 'skia_scalar=fixed'},
    gm_image_subdir='base-MacPro-fixed',
    perf_output_dir=os.path.join(
        perf_output_basedir_mac, 'Skia_Mac_Fixed_NoDebug'),
    ).Build())
B('Skia_Mac_Float_Debug', 'f_skia_mac_float_debug',
  scheduler='skia_rel')
F('f_skia_mac_float_debug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_MAC,
    configuration='Debug',
    environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
    gm_image_subdir='base',
    perf_output_dir=None, # no perf measurement for debug builds
    ).Build())
B('Skia_Mac_Float_NoDebug', 'f_skia_mac_float_nodebug',
  scheduler='skia_rel')
F('f_skia_mac_float_nodebug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_MAC,
    configuration='Release',
    environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
    gm_image_subdir='base',
    perf_output_dir=os.path.join(
        perf_output_basedir_mac, 'Skia_Mac_Float_NoDebug'),
    ).Build())

# Windows...
# TODO(epoger): for now, we don't do any perf measurement on Windows. Fix that.
B('Skia_Win32_Fixed_Debug', 'f_skia_win32_fixed_debug',
  scheduler='skia_rel')
F('f_skia_win32_fixed_debug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_WIN32,
    configuration='Debug',
    environment_variables={'GYP_DEFINES': 'skia_scalar=fixed'},
    gm_image_subdir='base-win',
    perf_output_dir=None, # no perf measurement for debug builds
    ).Build())
B('Skia_Win32_Fixed_NoDebug', 'f_skia_win32_fixed_nodebug',
  scheduler='skia_rel')
F('f_skia_win32_fixed_nodebug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_WIN32,
    configuration='Release',
    environment_variables={'GYP_DEFINES': 'skia_scalar=fixed'},
    gm_image_subdir='base-win',
    perf_output_dir=None, # TODO(epoger): no perf measurement for Windows builds
    ).Build())
B('Skia_Win32_Float_Debug', 'f_skia_win32_float_debug',
  scheduler='skia_rel')
F('f_skia_win32_float_debug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_WIN32,
    configuration='Debug',
    environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
    gm_image_subdir='base-win-fixed',
    perf_output_dir=None, # no perf measurement for debug builds
    ).Build())
B('Skia_Win32_Float_NoDebug', 'f_skia_win32_float_nodebug',
  scheduler='skia_rel')
F('f_skia_win32_float_nodebug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform=skia_factory.TARGET_PLATFORM_WIN32,
    configuration='Release',
    environment_variables={'GYP_DEFINES': 'skia_scalar=float'},
    gm_image_subdir='base-win-fixed',
    perf_output_dir=None, # TODO(epoger): no perf measurement for Windows builds
    ).Build())


def Update(config, active_master, c):
  return helper.Update(c)
