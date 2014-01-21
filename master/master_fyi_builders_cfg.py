# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want the FYI buildbot master to run.


from master_builders_cfg import f_xsan, LINUX
from skia_master_scripts import factory as skia_factory
from skia_master_scripts import moz2d_canary_factory
from skia_master_scripts import utils

import builder_name_schema
import master_builders_cfg


def setup_canaries(helper, do_upload_results):
  """Set up the Canary builders.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  # TODO(borenet): Specify the Housekeeping and Canary builders in a nice table
  # like the Build/Test/Perf builders.
  canaries = [
      (builder_name_schema.MakeBuilderName(role='Canary',
                                           project='Moz2D',
                                           os='Ubuntu12',
                                           compiler='GCC',
                                           target_arch='x86_64',
                                           configuration='Release'),
       skia_factory.TARGET_PLATFORM_LINUX,
       moz2d_canary_factory.Moz2DCanaryFactory,
       'skia_rel',
       {}),
  ]
  # Add corresponding trybot builders to the above list.
  canaries.extend([
      (builder + builder_name_schema.BUILDER_NAME_SEP + \
           builder_name_schema.TRYBOT_NAME_SUFFIX,
       target_platform,
       factory,
       utils.TRY_SCHEDULERS_STR,
       factory_args)
      for (builder, target_platform, factory, _scheduler,
           factory_args) in canaries])

  for (builder_name, target_platform, factory, scheduler,
       factory_args) in canaries:
    helper.Builder(builder_name, 'f_%s' % builder_name, scheduler=scheduler)
    helper.Factory('f_%s' % builder_name,
        factory(
            do_upload_results=do_upload_results,
            target_platform=target_platform,
            builder_name=builder_name,
            do_patch_step=(scheduler == utils.TRY_SCHEDULERS_STR),
            **factory_args
        ).Build())


def setup_compile_builders(helper, do_upload_results):
  """Set up all compile builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their
          results.
  """
  #
  #                            COMPILE BUILDERS
  #
  #    OS          Compiler  Config     Arch      Extra Config GYP_DEFS   WERR   Factory Args
  #
  builder_specs = [
      ('Ubuntu13', 'Clang',  'Debug',   'x86_64', 'TSAN',      None,      False, {'sanitizer': 'thread'}, f_xsan, LINUX)
  ]

  master_builders_cfg.setup_compile_builders_from_config_list(
      builder_specs, helper, do_upload_results)


def setup_test_and_perf_builders(helper, do_upload_results):
  """Set up all Test and Perf builders for the private master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  #
  #                            TEST AND PERF BUILDERS
  #
  #    Role    OS          Model          GPU       Arch      Config     Extra Config GYP_DEFS Factory Args
  #
  builder_specs = [
      ('Test', 'Ubuntu13', 'ShuttleA',   'HD2000',  'x86_64', 'Debug',   'TSAN',      None,    {'sanitizer': 'thread'}, f_xsan, LINUX),
  ]

  master_builders_cfg.setup_test_and_perf_builders_from_config_list(
      builder_specs, helper, do_upload_results)


def setup_all_builders(helper, do_upload_results):
  """Set up all builders for the FYI master.

  Args:
      helper: instance of utils.SkiaHelper
      do_upload_results: bool; whether the builders should upload their results.
  """
  setup_compile_builders(helper, do_upload_results)
  setup_test_and_perf_builders(helper, do_upload_results)
  setup_canaries(helper, do_upload_results)
