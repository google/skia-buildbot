# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Sets up all the builders we want this buildbot master to run.

#pylint: disable=C0301

from skia_master_scripts import android_factory
from skia_master_scripts import chromeos_factory
from skia_master_scripts import factory as skia_factory
from skia_master_scripts import housekeeping_percommit_factory
from skia_master_scripts import housekeeping_periodic_factory
from skia_master_scripts import ios_factory
from skia_master_scripts import nacl_factory
from skia_master_scripts import utils

import builder_name_schema


# Directory where we want to record performance data
#
# TODO(epoger): consider changing to reuse existing config.Master.perf_base_url,
# config.Master.perf_report_url_suffix, etc.
perf_output_basedir_linux = '../../../../perfdata'
perf_output_basedir_mac = perf_output_basedir_linux
perf_output_basedir_windows = '..\\..\\..\\..\\perfdata'

defaults = {}


ARCH_TO_GYP_DEFINE = {
  'x86': {'skia_arch_width': '32'},
  'x86_64': {'skia_arch_width': '64'},
  'Arm7': {'skia_arch_width': '32'},
  'NaCl': None,
}


CHROMEOS_BOARD_NAME = {
  'Alex': 'x86-alex',
  'Link': 'link',
  'Daisy': 'daisy',
}


def GetExtraFactoryArgs(compile_builder_info):
  factory_type = compile_builder_info[7]
  if factory_type == android_factory.AndroidFactory:
    # AndroidFactory requires a "device" argument.
    return {'device': utils.CapWordsToUnderscores(compile_builder_info[4])}
  elif factory_type == chromeos_factory.ChromeOSFactory:
    # ChromeOSFactory requires a "board" argument.
    try:
      return {'board': CHROMEOS_BOARD_NAME[compile_builder_info[4]],
              'bench_pictures_cfg': 'no_gpu'}
    except KeyError:
      raise Exception('Unknown board type "%s"' % compile_builder_info[4])
  elif factory_type == skia_factory.SkiaFactory:
    # Some "normal" factories require extra arguments.
    if compile_builder_info[4] == 'ANGLE':
      return {'gm_args': ['--config', 'angle'],
              'bench_args': ['--config', 'ANGLE'],
              'bench_pictures_cfg': 'angle'}
    elif compile_builder_info[4] == 'Valgrind':
      return {'flavor': 'valgrind'}
    else:
      return {}
  else:
    return {}


def Update(config, active_master, cfg):
  helper = utils.SkiaHelper(defaults)

  #
  # Default (per-commit) Scheduler for Skia. Only use this for builders which
  # do not care about commits outside of SKIA_PRIMARY_SUBDIRS.
  #
  helper.AnyBranchScheduler('skia_rel', branches=utils.SKIA_PRIMARY_SUBDIRS)

  #
  # Periodic Scheduler for Skia. The buildbot master follows UTC.
  # Setting it to 7AM UTC (2 AM EST).
  #
  helper.PeriodicScheduler('skia_periodic', branch='trunk', minute=0, hour=7)

  # Schedulers for Skia trybots.
  helper.TryJobSubversion(utils.TRY_SCHEDULER_SVN)
  helper.TryJobRietveld(utils.TRY_SCHEDULER_RIETVELD)

  #
  # Set up all the builders.
  #
  # Don't put spaces or 'funny characters' within the builder names, so that
  # we can safely use the builder name as part of a filepath.
  #
  do_upload_results = active_master.is_production_host

  gyp_win = repr({'skia_win_debuggers_path': 'c:/DbgHelp'})
  gyp_angle = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                    'skia_angle': '1'})
  gyp_dw = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                 'skia_directwrite': '1'})
  gyp_exc = repr({'skia_win_debuggers_path': 'c:/DbgHelp',
                  'skia_win_exceptions': '1'})
  gyp_10_6 = repr({'skia_osx_sdkroot': 'macosx10.6'})
  gyp_10_7 = repr({'skia_mesa': '1'})
  gyp_ios = repr({'skia_os': 'ios'})
  no_gpu = repr({'skia_gpu': '0'})
  valgrind = repr({'skia_release_optimization_level': '1'})

  # builder_specs is a dictionary whose keys are specifications for compile
  # builders and values are specifications for Test and Perf builders which will
  # eventually *depend* on those compile builders.
  builder_specs = {}
  #
  #                            COMPILE BUILDERS                                                                              TEST AND PERF BUILDERS
  #
  #    OS          Compiler  Config     Arch     Extra Config    GYP_DEFS   WERR             Role    OS          Model         GPU            Extra Config   GM Subdir
  #
  f = skia_factory.SkiaFactory
  p = skia_factory.TARGET_PLATFORM_LINUX
  builder_specs.update({
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    None,          None,      True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          'base-shuttle_ubuntu12_ati5770')],
      ('Ubuntu12', 'GCC',    'Release', 'x86',    None,          None,      True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          'base-shuttle_ubuntu12_ati5770'),
                                                                                            ('Perf', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', None,          None,      True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          'base-shuttle_ubuntu12_ati5770')],
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', None,          None,      True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          'base-shuttle_ubuntu12_ati5770'),
                                                                                            ('Perf', 'Ubuntu12', 'ShuttleA',   'ATI5770',     None,          None)],
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Valgrind',    valgrind,  False, f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'HD2000',      'Valgrind',    None)],
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'NoGPU',       no_gpu,    True,  f, p) : [('Test', 'Ubuntu12', 'ShuttleA',   'NoGPU',       None,          'base-shuttle_ubuntu12_ati5770')],
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'NoGPU',       no_gpu,    True,  f, p) : [],})
  f = nacl_factory.NaClFactory
  builder_specs.update({
      ('Ubuntu12', 'GCC',    'Debug',   'NaCl',   None,          None,      True,  f, p) : [],
      ('Ubuntu12', 'GCC',    'Release', 'NaCl',   None,          None,      True,  f, p) : [],})
  f = skia_factory.SkiaFactory
  p = skia_factory.TARGET_PLATFORM_MAC
  builder_specs.update({
      ('Mac10.6',  'GCC',    'Debug',   'x86',    None,          gyp_10_6,  True,  f, p) : [('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          'base-macmini')],
      ('Mac10.6',  'GCC',    'Release', 'x86',    None,          gyp_10_6,  True,  f, p) : [('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          'base-macmini'),
                                                                                            ('Perf', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.6',  'GCC',    'Debug',   'x86_64', None,          gyp_10_6,  False, f, p) : [('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          'base-macmini')],
      ('Mac10.6',  'GCC',    'Release', 'x86_64', None,          gyp_10_6,  False, f, p) : [('Test', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          'base-macmini'),
                                                                                            ('Perf', 'Mac10.6',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.7',  'Clang',  'Debug',   'x86',    None,          gyp_10_7,  True,  f, p) : [('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-lion-float')],
      ('Mac10.7',  'Clang',  'Release', 'x86',    None,          gyp_10_7,  True,  f, p) : [('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-lion-float'),
                                                                                            ('Perf', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.7',  'Clang',  'Debug',   'x86_64', None,          gyp_10_7,  False, f, p) : [('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-lion-float')],
      ('Mac10.7',  'Clang',  'Release', 'x86_64', None,          gyp_10_7,  False, f, p) : [('Test', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-lion-float'),
                                                                                            ('Perf', 'Mac10.7',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.8',  'Clang',  'Debug',   'x86',    None,          None,      True,  f, p) : [('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-10_8')],
      ('Mac10.8',  'Clang',  'Release', 'x86',    None,          None,      True,  f, p) : [('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-10_8'),
                                                                                            ('Perf', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          None)],
      ('Mac10.8',  'Clang',  'Debug',   'x86_64', None,          None,      False, f, p) : [('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-10_8')],
      ('Mac10.8',  'Clang',  'Release', 'x86_64', None,          None,      False, f, p) : [('Test', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          'base-macmini-10_8'),
                                                                                            ('Perf', 'Mac10.8',  'MacMini4.1', 'GeForce320M', None,          None)],})
  p = skia_factory.TARGET_PLATFORM_WIN32
  builder_specs.update({
      ('Win7',     'VS2010', 'Debug',   'x86',    None,          gyp_win,   True,  f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      None,          'base-shuttle-win7-intel-float')],
      ('Win7',     'VS2010', 'Release', 'x86',    None,          gyp_win,   True,  f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      None,          'base-shuttle-win7-intel-float'),
                                                                                            ('Perf', 'Win7',     'ShuttleA',   'HD2000',      None,          None)],
      ('Win7',     'VS2010', 'Debug',   'x86_64', None,          gyp_win,   False, f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      None,          'base-shuttle-win7-intel-float')],
      ('Win7',     'VS2010', 'Release', 'x86_64', None,          gyp_win,   False, f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      None,          'base-shuttle-win7-intel-float'),
                                                                                            ('Perf', 'Win7',     'ShuttleA',   'HD2000',      None,          None)],
      ('Win7',     'VS2010', 'Debug',   'x86',    'ANGLE',       gyp_angle, True,  f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      'ANGLE',       'base-shuttle-win7-intel-angle')],
      ('Win7',     'VS2010', 'Release', 'x86',    'ANGLE',       gyp_angle, True,  f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      'ANGLE',       'base-shuttle-win7-intel-angle'),
                                                                                            ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'ANGLE',       None)],
      ('Win7',     'VS2010', 'Debug',   'x86',    'DirectWrite', gyp_dw,    False, f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      'DirectWrite', 'base-shuttle-win7-intel-directwrite')],
      ('Win7',     'VS2010', 'Release', 'x86',    'DirectWrite', gyp_dw,    False, f, p) : [('Test', 'Win7',     'ShuttleA',   'HD2000',      'DirectWrite', 'base-shuttle-win7-intel-directwrite'),
                                                                                            ('Perf', 'Win7',     'ShuttleA',   'HD2000',      'DirectWrite', None)],
      ('Win7',     'VS2010', 'Debug',   'x86',    'Exceptions',  gyp_exc,   False, f, p) : [],
      ('Win8',     'VS2012', 'Debug',   'x86',    None,          gyp_win,   True,  f, p) : [],
      ('Win8',     'VS2012', 'Release', 'x86',    None,          gyp_win,   True,  f, p) : [],
      ('Win8',     'VS2012', 'Debug',   'x86_64', None,          gyp_win,   False, f, p) : [],
      ('Win8',     'VS2012', 'Release', 'x86_64', None,          gyp_win,   False, f, p) : []})
  f = android_factory.AndroidFactory
  p = skia_factory.TARGET_PLATFORM_LINUX
  builder_specs.update({
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'NexusS',      None,      True,  f, p) : [('Test', 'Android',  'NexusS',     'SGX540',      None,          'base-android-nexus-s')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'NexusS',      None,      True,  f, p) : [('Test', 'Android',  'NexusS',     'SGX540',      None,          'base-android-nexus-s'),
                                                                                            ('Perf', 'Android',  'NexusS',     'SGX540',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus4',      None,      True,  f, p) : [('Test', 'Android',  'Nexus4',     'Adreno320',   None,          'base-android-nexus-4')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus4',      None,      True,  f, p) : [('Test', 'Android',  'Nexus4',     'Adreno320',   None,          'base-android-nexus-4'),
                                                                                            ('Perf', 'Android',  'Nexus4',     'Adreno320',   None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus7',      None,      True,  f, p) : [('Test', 'Android',  'Nexus7',     'Tegra3',      None,          'base-android-nexus-7')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus7',      None,      True,  f, p) : [('Test', 'Android',  'Nexus7',     'Tegra3',      None,          'base-android-nexus-7'),
                                                                                            ('Perf', 'Android',  'Nexus7',     'Tegra3',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Nexus10',     None,      True,  f, p) : [('Test', 'Android',  'Nexus10',    'MaliT604',    None,          'base-android-nexus-10')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Nexus10',     None,      True,  f, p) : [('Test', 'Android',  'Nexus10',    'MaliT604',    None,          'base-android-nexus-10'),
                                                                                            ('Perf', 'Android',  'Nexus10',    'MaliT604',    None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'GalaxyNexus', None,      True,  f, p) : [('Test', 'Android',  'GalaxyNexus','SGX540',      None,          'base-android-galaxy-nexus')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'GalaxyNexus', None,      True,  f, p) : [('Test', 'Android',  'GalaxyNexus','SGX540',      None,          'base-android-galaxy-nexus'),
                                                                                            ('Perf', 'Android',  'GalaxyNexus','SGX540',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Xoom',        None,      True,  f, p) : [('Test', 'Android',  'Xoom',       'Tegra2',      None,          'base-android-xoom')],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Xoom',        None,      True,  f, p) : [('Test', 'Android',  'Xoom',       'Tegra2',      None,          'base-android-xoom'),
                                                                                            ('Perf', 'Android',  'Xoom',       'Tegra2',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'RazrI',       None,      True,  f, p) : [('Test', 'Android',  'RazrI',      'SGX540',      None,          'base-android-razr-i')],
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'RazrI',       None,      True,  f, p) : [('Test', 'Android',  'RazrI',      'SGX540',      None,          'base-android-razr-i'),
                                                                                            ('Perf', 'Android',  'RazrI',      'SGX540',      None,          None)],})
  f = chromeos_factory.ChromeOSFactory
  builder_specs.update({
      ('Ubuntu12', 'GCC',    'Debug',   'x86',    'Alex',        None,      True,  f, p) : [('Test', 'ChromeOS', 'Alex',       'GMA3150',     None,          None)],
      ('Ubuntu12', 'GCC',    'Release', 'x86',    'Alex',        None,      True,  f, p) : [('Test', 'ChromeOS', 'Alex',       'GMA3150',     None,          None),
                                                                                            ('Perf', 'ChromeOS', 'Alex',       'GMA3150',     None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'x86_64', 'Link',        None,      True,  f, p) : [('Test', 'ChromeOS', 'Link',       'HD4000',      None,          None)],
      ('Ubuntu12', 'GCC',    'Release', 'x86_64', 'Link',        None,      True,  f, p) : [('Test', 'ChromeOS', 'Link',       'HD4000',      None,          None),
                                                                                            ('Perf', 'ChromeOS', 'Link',       'HD4000',      None,          None)],
      ('Ubuntu12', 'GCC',    'Debug',   'Arm7',   'Daisy',       None,      True,  f, p) : [('Test', 'ChromeOS', 'Daisy',      'MaliT604',    None,          None)],
      ('Ubuntu12', 'GCC',    'Release', 'Arm7',   'Daisy',       None,      True,  f, p) : [('Test', 'ChromeOS', 'Daisy',      'MaliT604',    None,          None),
                                                                                            ('Perf', 'ChromeOS', 'Daisy',      'MaliT604',    None,          None)],})
  f = ios_factory.iOSFactory
  p = skia_factory.TARGET_PLATFORM_MAC
  builder_specs.update({
      ('Mac10.7',  'Clang',  'Debug',   'Arm7',   'iOS',         gyp_ios,   True,  f, p) : [],
      ('Mac10.7',  'Clang',  'Release', 'Arm7',   'iOS',         gyp_ios,   True,  f, p) : [],})

  for compile_builder in sorted(builder_specs.keys()):
    factory_type = compile_builder[7]
    factory_args = GetExtraFactoryArgs(compile_builder)
    target_platform = compile_builder[8]
    try:
      arch_width_define = ARCH_TO_GYP_DEFINE[compile_builder[3]]
    except KeyError:
      raise Exception('Unknown arch type: %s' % compile_builder[3])
    gyp_defines = eval(compile_builder[5] or 'None')
    if arch_width_define:
      if not gyp_defines:
        gyp_defines = arch_width_define
      else:
        if 'skia_arch_width' in gyp_defines.keys():
          raise ValueError('Cannot define skia_arch_width; it is derived from '
                           'the provided arch type.')
        gyp_defines.update(arch_width_define)
    utils.MakeCompileBuilderSet(
        helper=helper,
        scheduler='skia_rel',
        os=compile_builder[0],
        compiler=compile_builder[1],
        configuration=compile_builder[2],
        target_arch=compile_builder[3],
        extra_config=compile_builder[4],
        gyp_defines=gyp_defines,
        do_upload_results=do_upload_results,
        compile_warnings_as_errors=compile_builder[6],
        factory_type=factory_type,
        target_platform=target_platform,
        **factory_args)
    for dependent_builder in builder_specs[compile_builder]:
      role = dependent_builder[0]
      perf_output_basedir = None
      if role == builder_name_schema.BUILDER_ROLE_PERF:
        if target_platform == skia_factory.TARGET_PLATFORM_LINUX:
          perf_output_basedir = perf_output_basedir_linux
        elif target_platform == skia_factory.TARGET_PLATFORM_MAC:
          perf_output_basedir = perf_output_basedir_mac
        elif target_platform == skia_factory.TARGET_PLATFORM_WIN32:
          perf_output_basedir = perf_output_basedir_windows
      utils.MakeBuilderSet(
          helper=helper,
          role=role,
          os=dependent_builder[1],
          model=dependent_builder[2],
          gpu=dependent_builder[3],
          extra_config=dependent_builder[4],
          configuration=compile_builder[2],
          arch=compile_builder[3],
          gyp_defines=gyp_defines,
          factory_type=factory_type,
          target_platform=target_platform,
          gm_image_subdir=dependent_builder[5],
          do_upload_results=do_upload_results,
          perf_output_basedir=perf_output_basedir,
          compile_warnings_as_errors=False,
          **factory_args)

  # House Keeping
  housekeepers = [
    # The Percommit housekeeper
    (builder_name_schema.MakeBuilderName(role='Housekeeper',
                                         frequency='PerCommit'),
     housekeeping_percommit_factory.HouseKeepingPerCommitFactory,
     'skia_rel'),
    # The Periodic housekeeper
    (builder_name_schema.MakeBuilderName(role='Housekeeper',
                                         frequency='Nightly'),
     housekeeping_periodic_factory.HouseKeepingPeriodicFactory,
     'skia_periodic'),
  ]
  # Add the corresponding trybot builders to the above list.
  housekeepers.extend([
      (builder + builder_name_schema.BUILDER_NAME_SEP + \
       builder_name_schema.TRYBOT_NAME_SUFFIX, factory,
       utils.TRY_SCHEDULERS_STR)
      for (builder, factory, _scheduler) in housekeepers])

  for (builder_name, factory, scheduler) in housekeepers:
    helper.Builder(builder_name, 'f_%s' % builder_name, scheduler=scheduler)
    helper.Factory('f_%s' % builder_name,
      factory(
        do_upload_results=do_upload_results,
        target_platform=skia_factory.TARGET_PLATFORM_LINUX,
        builder_name=builder_name,
        do_patch_step=(scheduler == utils.TRY_SCHEDULERS_STR),
      ).Build())

  return helper.Update(cfg)
