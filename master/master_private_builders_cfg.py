# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the private buildbot master to run.


from skia_master_scripts import android_factory
from skia_master_scripts import factory as skia_factory

import master_builders_cfg
from master_builders_cfg import GYP_NVPR


def setup_all_builders(helper, do_upload_results):
  """Set up all builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  # builder_specs is a dictionary whose keys are specifications for compile
  # builders and values are specifications for Test and Perf builders which will
  # eventually *depend* on those compile builders.
  builder_specs = {}
  #
  #                            COMPILE BUILDERS                                                                              TEST AND PERF BUILDERS
  #
  #    OS          Compiler  Config     Arch     Extra Config    GYP_DEFS   WERR             Role    OS          Model         GPU            Extra Config   GM Subdir
  #
  f = android_factory.AndroidFactory
  p = skia_factory.TARGET_PLATFORM_LINUX
  builder_specs.update({
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',  'NvidiaLogan',  GYP_NVPR,  True,  f, p) : [('Test', 'Android',  'Logan',      'Nvidia',      None,          'base-android-logan')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',  'NvidiaLogan',  GYP_NVPR,  True,  f, p) : [('Test', 'Android',  'Logan',      'Nvidia',      None,          'base-android-logan'),
                                                                                            ('Perf', 'Android',  'Logan',      'Nvidia',      None,          None)],
  })

  master_builders_cfg.setup_builders_from_config_dict(builder_specs, helper,
                                                      do_upload_results)
