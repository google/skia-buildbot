# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from master import master_config
from master.factory import skia_factory

defaults = {}

helper = master_config.Helper(defaults)
B = helper.Builder
D = helper.Dependent
F = helper.Factory
S = helper.Scheduler

defaults['category'] = 'linux'


#
# Main Scheduler for Skia
#
S('skia_rel', branch='trunk', treeStableTimer=60)


#
# Linux Release Builder
#
B('Skia Linux Fixed Debug', 'f_skia_linux_fixed_debug',
  scheduler='skia_rel')
F('f_skia_linux_fixed_debug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform='linux',
    buildtype='Debug', additional_gyp_args='-Dskia_scalar=fixed',
    gm_image_subdir='base-linux-fixed',
    ).Build())
B('Skia Linux Fixed NoDebug', 'f_skia_linux_fixed_nodebug',
  scheduler='skia_rel')
F('f_skia_linux_fixed_nodebug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform='linux',
    buildtype='Release', additional_gyp_args='-Dskia_scalar=fixed',
    gm_image_subdir='base-linux-fixed',
    ).Build())
B('Skia Linux Float Debug', 'f_skia_linux_float_debug',
  scheduler='skia_rel')
F('f_skia_linux_float_debug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform='linux',
    buildtype='Debug', additional_gyp_args='-Dskia_scalar=float',
    gm_image_subdir='base-linux',
    ).Build())
B('Skia Linux Float NoDebug', 'f_skia_linux_float_nodebug',
  scheduler='skia_rel')
F('f_skia_linux_float_nodebug', skia_factory.SkiaFactory(
    build_subdir='Skia', target_platform='linux',
    buildtype='Release', additional_gyp_args='-Dskia_scalar=float',
    gm_image_subdir='base-linux',
    ).Build())


def Update(config, active_master, c):
  return helper.Update(c)
