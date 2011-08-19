# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from master import master_config
from master.factory import chromium_factory

defaults = {}

helper = master_config.Helper(defaults)
B = helper.Builder
D = helper.Dependent
F = helper.Factory
S = helper.Scheduler
T = helper.Triggerable

def mac(): return chromium_factory.ChromiumFactory('src/build', 'darwin')

defaults['category'] = 'mac latest'
all_our_tests = ['base', 'browser_tests', 'cacheinvalidation', 'crypto',
                 'googleurl', 'gpu', 'jingle', 'media', 'nacl_integration',
                 'nacl_sandbox', 'nacl_ui', 'page_cycler', 'printing',
                 'remoting', 'safe_browsing', 'ui',]

################################################################################
## Debug
################################################################################

#
# Main debug scheduler for Chromium
#
S('s_chromium_dbg', branch='src', treeStableTimer=60)

#
# Triggerable schedulers for the dbg builders
#
T('mac_skia_dbg_trigger')
T('mac_noskia_dbg_trigger')


#
# Combination builders/testers for Mac OS 10.6
# (These archive their builds for testers to run, too.)
#

skia_dbg_archive = master_config.GetArchiveUrl(
    'ChromiumFYI', 'Chromium Mac 10.6 Skia (dbg)',
    'cr-mac-skia-dbg', 'mac')
B('Chromium Mac 10.6 Skia (dbg)', 'f_chromium_mac_10_6_skia_dbg',
  scheduler='s_chromium_dbg', builddir='cr-mac-skia-dbg')
F('f_chromium_mac_10_6_skia_dbg', mac().ChromiumFactory(
    slave_type='Builder',
    target='Debug',
    factory_properties={
        'trigger': 'mac_skia_dbg_trigger',
        'gclient_env': { 'GYP_DEFINES':'use_skia=1'},
    },
    tests=all_our_tests,
))

noskia_dbg_archive = master_config.GetArchiveUrl(
    'ChromiumFYI', 'Chromium Mac 10.6 NoSkia (dbg)',
    'cr-mac-noskia-dbg', 'mac')
B('Chromium Mac 10.6 NoSkia (dbg)', 'f_chromium_mac_10_6_noskia_dbg',
  scheduler='s_chromium_dbg', builddir='cr-mac-noskia-dbg')
F('f_chromium_mac_10_6_noskia_dbg', mac().ChromiumFactory(
    slave_type='Builder',
    target='Debug',
    factory_properties={
        'trigger': 'mac_noskia_dbg_trigger',
    },
    tests=all_our_tests,
))


#
# Testers for Mac OS 10.5
# (These run the archived builds created by the builder/testers above.)
#

B('Chromium Mac 10.5 Skia (dbg)', 'f_chromium_mac_10_5_skia_dbg',
  scheduler='mac_skia_dbg_trigger')
F('f_chromium_mac_10_5_skia_dbg', mac().ChromiumFactory(
    slave_type='Tester',
    build_url=skia_dbg_archive,
    factory_properties={
        'generate_gtest_json': True,
    },
    tests=all_our_tests,
))

B('Chromium Mac 10.5 NoSkia (dbg)', 'f_chromium_mac_10_5_noskia_dbg',
  scheduler='mac_noskia_dbg_trigger')
F('f_chromium_mac_10_5_noskia_dbg', mac().ChromiumFactory(
    slave_type='Tester',
    build_url=noskia_dbg_archive,
    factory_properties={
        'generate_gtest_json': True,
    },
    tests=all_our_tests,
))



def Update(config, active_master, c):
  return helper.Update(c)
