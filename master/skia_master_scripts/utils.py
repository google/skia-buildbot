# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Miscellaneous utilities needed by the Skia buildbot master."""

import httplib2

# requires Google APIs client library for Python; see
# https://code.google.com/p/google-api-python-client/wiki/Installation
from apiclient.discovery import build
from buildbot.scheduler import AnyBranchScheduler
from buildbot.scheduler import Scheduler
from buildbot.schedulers import timed
from buildbot.util import NotABranch
from master import master_config
from oauth2client.client import SignedJwtAssertionCredentials
from skia_master_scripts import android_factory
from skia_master_scripts import factory as skia_factory
from skia_master_scripts.perf_only_factory import PerfOnlyFactory, AndroidPerfOnlyFactory
from skia_master_scripts.no_perf_factory import NoPerfFactory, AndroidNoPerfFactory

def _AssertValidString(var, varName='[unknown]'):
  """Raises an exception if a var is not a valid string.
  
  A string is considered valid if it is not None, is not the empty string and is
  not just whitespace.

  Args:
    var: the variable to validate
    varName: name of the variable, for error reporting
  """
  if not isinstance(var, str):
    raise Exception('variable "%s" is not a string' % varName)
  if not var:
    raise Exception('variable "%s" is empty' % varName)
  if var.isspace():
    raise Exception('variable "%s" is whitespace' % varName)

def _AssertValidStringList(var, varName='[unknown]'):
  """Raises an exception if var is not a list of valid strings.
  
  A list is considered valid if it is either empty or if it contains at
  least one item and each item it contains is also a valid string.

  Args:
    var: the variable to validate
    varName: name of the variable, for error reporting
  """
  if not isinstance(var, list):
    raise Exception('variable "%s" is not a list' % varName)
  for index, item in zip(range(len(var)), var):
    _AssertValidString(item, '%s[%d]' % (varName, index))

def FileBug(summary, description, owner=None, ccs=[], labels=[]):
  """Files a bug to the Skia issue tracker.

  Args:
    summary: a single-line string to use as the issue summary
    description: a multiline string to use as the issue description
    owner: email address of the issue owner (as a string), or None if unknown
    ccs: email addresses (list of strings) to CC on the bug
    labels: labels (list of strings) to apply to the bug

  Returns: 
    A representation of the issue tracker issue that was filed or raises an
    exception if there was a problem.
  """
  project_id = 'skia' # This is the project name: skia
  key_file = 'key.p12' # Key file from the API console, renamed to key.p12
  service_acct = ('352371350305-b3u8jq5sotdh964othi9ntg9d0pelu77'
                  '@developer.gserviceaccount.com') # Created with the key
  result = {} 
  
  if owner is not None:  # owner can be None
    _AssertValidString(owner, 'owner')
  _AssertValidString(summary, 'summary')
  _AssertValidString(description, 'description')
  _AssertValidStringList(ccs, 'ccs')
  _AssertValidStringList(labels, 'labels')

  f = file(key_file, 'rb')
  key = f.read()
  f.close()

  # Create an httplib2.Http object to handle the HTTP requests and authorize
  # it with the credentials.
  credentials = SignedJwtAssertionCredentials(
      service_acct,
      key,
      scope='https://www.googleapis.com/auth/projecthosting')
  http = httplib2.Http()
  http = credentials.authorize(http)

  service = build("projecthosting", "v2", http=http)

  # Insert a new issue into the project.
  body = {
    'summary': summary,
    'description': description
  }
  
  insertparams = {
    'projectId': project_id,
    'sendEmail': 'true'
  }

  if owner is not None:
    owner_value = {
      'name': owner
    }
    body['owner'] = owner_value

  cc_values = []
  for cc in ccs:
    cc_values.append({'name': cc})  
  body['cc'] = cc_values
  
  body['labels'] = labels

  insertparams['body'] = body
   
  request = service.issues().insert(**insertparams)
  result = request.execute()

  return result

# Branches for which we trigger rebuilds on the primary builders
SKIA_PRIMARY_SUBDIRS = ['android', 'buildbot', 'gm-expected', 'trunk']

# Since we can't modify the existing Helper class, we subclass it here,
# overriding the necessary parts to get things working as we want.
# Specifically, the Helper class hardcodes each registered scheduler to be
# instantiated as a 'Scheduler,' which aliases 'SingleBranchScheduler.'  We add
# an 'AnyBranchScheduler' method and change the implementation of Update() to
# instantiate the proper type.

# TODO(borenet): modify this code upstream so that we don't need this override.	
# BUG: http://code.google.com/p/skia/issues/detail?id=761
class SkiaHelper(master_config.Helper):
  def Builder(self, name, factory, gatekeeper=None, scheduler=None,
              builddir=None, auto_reboot=False, notify_on_missing=False):
    super(SkiaHelper, self).Builder(name=name, factory=factory,
                                    gatekeeper=gatekeeper, scheduler=scheduler,
                                    builddir=builddir, auto_reboot=auto_reboot,
                                    notify_on_missing=notify_on_missing)

  def AnyBranchScheduler(self, name, branches, treeStableTimer=60,
                         categories=None):
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exist' % name)
    self._schedulers[name] = {'type': 'AnyBranchScheduler',
                              'branches': branches,
                              'treeStableTimer': treeStableTimer,
                              'builders': [],
                              'categories': categories}

  def PeriodicScheduler(self, name, branch, minute=0, hour='*', dayOfMonth='*',
                        month='*', dayOfWeek='*'):
    """Configures the periodic build scheduler.

    The timezone the PeriodicScheduler is run in is the timezone of the buildbot
    master. Currently this is EST because it runs in Atlanta.
    """
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exist' % name)
    self._schedulers[name] = {'type': 'PeriodicScheduler',
                              'branch': branch,
                              'builders': [],
                              'minute': minute,
                              'hour': hour,
                              'dayOfMonth': dayOfMonth,
                              'month': month,
                              'dayOfWeek': dayOfWeek}

  def Update(self, c):
    super(SkiaHelper, self).Update(c)
    for s_name in self._schedulers:
      scheduler = self._schedulers[s_name]
      instance = None
      if scheduler['type'] == 'AnyBranchScheduler':
        instance = AnyBranchScheduler(name=s_name,
                                      branch=NotABranch,
                                      branches=scheduler['branches'],
                                      treeStableTimer=
                                          scheduler['treeStableTimer'],
                                      builderNames=scheduler['builders'],
                                      categories=scheduler['categories'])
      elif scheduler['type'] == 'PeriodicScheduler':
        instance = timed.Nightly(name=s_name,
                                 branch=scheduler['branch'],
                                 builderNames=scheduler['builders'],
                                 minute=scheduler['minute'],
                                 hour=scheduler['hour'],
                                 dayOfMonth=scheduler['dayOfMonth'],
                                 month=scheduler['month'],
                                 dayOfWeek=scheduler['dayOfWeek'])
      else:
        raise ValueError(
            'The scheduler type is unrecognized %s' % scheduler['type'])
      scheduler['instance'] = instance
      c['schedulers'].append(instance)

def MakeBuilderName(builder_base_name, config):
  """ Inserts config into builder_base_name at '%s', or if builder_base_name
  does not contain '%s', appends config to the end of builder_base_name,
  separated by an underscore. """
  try:
    return builder_base_name % config
  except TypeError:
    # If builder_base_name does not contain '%s'
    return '%s_%s' % (builder_base_name, config)

def MakeDebugBuilderName(builder_base_name):
  return MakeBuilderName(builder_base_name, skia_factory.CONFIG_DEBUG)

def MakeReleaseBuilderName(builder_base_name):
  return MakeBuilderName(builder_base_name, skia_factory.CONFIG_RELEASE)

def MakeBenchBuilderName(builder_base_name):
  return MakeBuilderName(builder_base_name, skia_factory.CONFIG_BENCH)

def MakeBuilderSet(helper, scheduler, builder_base_name, do_upload_results,
                   target_platform, environment_variables, gm_image_subdir,
                   perf_output_basedir, test_args=None, gm_args=None,
                   bench_args=None):
  """ Creates a trio of builders for a given platform:
  1. Debug mode builder which runs all steps
  2. Release mode builder which runs all steps EXCEPT benchmarks
  3. Release mode builder which runs ONLY benchmarks.
  """
  B = helper.Builder
  F = helper.Factory

  debug_builder_name   = MakeDebugBuilderName(builder_base_name)
  no_perf_builder_name = MakeReleaseBuilderName(builder_base_name)
  perf_builder_name    = MakeBenchBuilderName(builder_base_name)

  B(debug_builder_name, 'f_%s' % debug_builder_name,
      scheduler=scheduler)
  F('f_%s' % debug_builder_name, skia_factory.SkiaFactory(
      do_upload_results=do_upload_results,
      target_platform=target_platform,
      configuration=skia_factory.CONFIG_DEBUG,
      environment_variables=environment_variables,
      gm_image_subdir=gm_image_subdir,
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name=debug_builder_name,
      test_args=test_args,
      gm_args=gm_args,
      bench_args=bench_args,
      ).Build())
  B(no_perf_builder_name, 'f_%s' % no_perf_builder_name,
      scheduler=scheduler)
  F('f_%s' % no_perf_builder_name,  NoPerfFactory(
      do_upload_results=do_upload_results,
      target_platform=target_platform,
      configuration=skia_factory.CONFIG_RELEASE,
      environment_variables=environment_variables,
      gm_image_subdir=gm_image_subdir,
      perf_output_basedir=None,
      builder_name=no_perf_builder_name,
      test_args=test_args,
      gm_args=gm_args,
      bench_args=bench_args,
      ).Build())
  B(perf_builder_name, 'f_%s' % perf_builder_name,
      scheduler=scheduler)
  F('f_%s' % perf_builder_name, PerfOnlyFactory(
      do_upload_results=do_upload_results,
      target_platform=target_platform,
      configuration=skia_factory.CONFIG_RELEASE,
      environment_variables=environment_variables,
      gm_image_subdir=gm_image_subdir,
      perf_output_basedir=perf_output_basedir,
      builder_name=perf_builder_name,
      test_args=test_args,
      gm_args=gm_args,
      bench_args=bench_args,
      ).Build())

def MakeAndroidBuilderSet(helper, scheduler, builder_base_name, device,
                          do_upload_results, target_platform, 
                          environment_variables, gm_image_subdir,
                          perf_output_basedir, serial=None, test_args=None,
                          gm_args=None, bench_args=None):
  """ Creates a trio of builders for Android:
  1. Debug mode builder which runs all steps
  2. Release mode builder which runs all steps EXCEPT benchmarks
  3. Release mode builder which runs ONLY benchmarks.
  """
  B = helper.Builder
  F = helper.Factory

  debug_builder_name   = MakeDebugBuilderName(builder_base_name)
  no_perf_builder_name = MakeReleaseBuilderName(builder_base_name)
  perf_builder_name    = MakeBenchBuilderName(builder_base_name)

  B(debug_builder_name, 'f_%s' % debug_builder_name,
      scheduler=scheduler)
  F('f_%s' % debug_builder_name, android_factory.AndroidFactory(
      device=device,
      serial=serial,
      do_upload_results=do_upload_results,
      target_platform=target_platform,
      configuration=skia_factory.CONFIG_DEBUG,
      environment_variables=environment_variables,
      gm_image_subdir=gm_image_subdir,
      perf_output_basedir=None, # no perf measurement for debug builds
      builder_name=debug_builder_name,
      test_args=test_args,
      gm_args=gm_args,
      bench_args=bench_args,
      ).Build())
  B(no_perf_builder_name, 'f_%s' % no_perf_builder_name,
      scheduler=scheduler)
  F('f_%s' % no_perf_builder_name,  AndroidNoPerfFactory(
      device=device,
      serial=serial,
      do_upload_results=do_upload_results,
      target_platform=target_platform,
      configuration=skia_factory.CONFIG_RELEASE,
      environment_variables=environment_variables,
      gm_image_subdir=gm_image_subdir,
      perf_output_basedir=None,
      builder_name=no_perf_builder_name,
      test_args=test_args,
      gm_args=gm_args,
      bench_args=bench_args,
      ).Build())
  B(perf_builder_name, 'f_%s' % perf_builder_name,
      scheduler=scheduler)
  F('f_%s' % perf_builder_name, AndroidPerfOnlyFactory(
      device=device,
      serial=serial,
      do_upload_results=do_upload_results,
      target_platform=target_platform,
      configuration=skia_factory.CONFIG_RELEASE,
      environment_variables=environment_variables,
      gm_image_subdir=gm_image_subdir,
      perf_output_basedir=perf_output_basedir,
      builder_name=perf_builder_name,
      test_args=test_args,
      gm_args=gm_args,
      bench_args=bench_args,
      ).Build())
