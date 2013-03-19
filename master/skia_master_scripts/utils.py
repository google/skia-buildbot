# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Miscellaneous utilities needed by the Skia buildbot master."""


import httplib2
import re


# requires Google APIs client library for Python; see
# https://code.google.com/p/google-api-python-client/wiki/Installation
from apiclient.discovery import build
from buildbot.scheduler import AnyBranchScheduler
from buildbot.schedulers import timed
from buildbot.schedulers.filter import ChangeFilter
from buildbot.util import NotABranch
from config_private import TRY_SVN_BASEURL
from master import master_config
from master.builders_pools import BuildersPools
from master import try_job_svn
from master import try_job_rietveld
from oauth2client.client import SignedJwtAssertionCredentials
from skia_master_scripts import android_factory
from skia_master_scripts import chromeos_factory
from skia_master_scripts import factory as skia_factory
from skia_master_scripts import housekeeping_percommit_factory, \
                                housekeeping_periodic_factory
from skia_master_scripts import ios_factory
import config_private


TRYBOT_NAME_SUFFIX = '_Trybot'
TRY_SCHEDULER_SVN = 'skia_try_svn'
TRY_SCHEDULER_RIETVELD = 'skia_try_rietveld'
TRY_SCHEDULERS = [TRY_SCHEDULER_SVN, TRY_SCHEDULER_RIETVELD]
TRY_SCHEDULERS_STR = '|'.join(TRY_SCHEDULERS)


def IsTrybot(builder_name):
  return builder_name.endswith(TRYBOT_NAME_SUFFIX)


class SkiaChangeFilter(ChangeFilter):
  """Skia specific subclass of ChangeFilter."""

  def __init__(self, builders, **kwargs):
    self._builders = builders
    ChangeFilter.__init__(self, **kwargs)

  def filter_change(self, change):
    """Overrides ChangeFilter.filter_change to pass builders to filter_fn.

    The code has been copied from
    http://buildbot.net/buildbot/docs/0.8.3/reference/buildbot.schedulers.filter-pysrc.html#ChangeFilter
    with one change: We pass a sequence of builders to the filter function.
    """
    if self.filter_fn is not None and not self.filter_fn(change,
                                                         self._builders):
      return False
    for (filt_list, filt_re, filt_fn, chg_attr) in self.checks:
      chg_val = getattr(change, chg_attr, '')
      if filt_list is not None and chg_val not in filt_list:
        return False
      if filt_re is not None and (
          chg_val is None or not filt_re.match(chg_val)):
        return False
      if filt_fn is not None and not filt_fn(chg_val):
        return False
    return True


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


def FileBug(summary, description, owner=None, ccs=None, labels=None):
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
  if not ccs:
    ccs = []
  if not labels:
    labels = []
  
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


# Base set of branches for which we trigger rebuilds on all builders. Schedulers
# may trigger builds on a superset of this list to include, for example, the
# 'android' branch or a subfolder of 'gm-expected'.
SKIA_PRIMARY_SUBDIRS = ['buildbot', 'skp', 'trunk']


# Skip buildbot runs of a CL if its commit log message contains the following
# substring.
SKIP_BUILDBOT_SUBSTRING = '(SkipBuildbotRuns)'

# If the below regex is found in a CL's commit log message, only run the
# builders specified therein.
RUN_BUILDERS_REGEX = '\(RunBuilders:(.+)\)'
RUN_BUILDERS_RE_COMPILED = re.compile(RUN_BUILDERS_REGEX)


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

  def TryJobSubversion(self, name):
    """ Adds a Subversion-based try scheduler. """
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'TryJobSubversion', 'builders': []}

  def TryJobRietveld(self, name):
    """ Adds a Rietveld-based try scheduler. """
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'TryJobRietveld', 'builders': []}

  def Update(self, c):
    super(SkiaHelper, self).Update(c)
    all_subdirs = SKIA_PRIMARY_SUBDIRS
    for s_name in self._schedulers:
      scheduler = self._schedulers[s_name]
      instance = None
      if scheduler['type'] == 'AnyBranchScheduler':
        def filter_fn(change, builders):
          """Filters out if change.comments contains certain keywords.

          The change is filtered out if the commit message contains:
          * SKIP_BUILDBOT_SUBSTRING or
          * RUN_BUILDERS_REGEX when the scheduler does not contain any of the
            specified builders

          Args:
            change: An instance of changes.Change.
            builders: Sequence of strings. The builders that are run by this
              scheduler.

          Returns:
            If the change should be filtered out (i.e. not run by the buildbot
            code) then False is returned else True is returned.
          """
          if SKIP_BUILDBOT_SUBSTRING in change.comments:
            return False
          match_obj = RUN_BUILDERS_RE_COMPILED.search(change.comments)
          if builders and match_obj:
            for builder_to_run in match_obj.group(1).split(','):
              if builder_to_run.strip() in builders:
                break
            else:
              return False
          return True

        skia_change_filter = SkiaChangeFilter(
            builders=scheduler['builders'], branch=scheduler['branches'],
            filter_fn=filter_fn)

        instance = AnyBranchScheduler(name=s_name,
                                      branch=NotABranch,
                                      branches=NotABranch,
                                      treeStableTimer=
                                          scheduler['treeStableTimer'],
                                      builderNames=scheduler['builders'],
                                      categories=scheduler['categories'],
                                      change_filter=skia_change_filter)
      elif scheduler['type'] == 'PeriodicScheduler':
        instance = timed.Nightly(name=s_name,
                                 branch=scheduler['branch'],
                                 builderNames=scheduler['builders'],
                                 minute=scheduler['minute'],
                                 hour=scheduler['hour'],
                                 dayOfMonth=scheduler['dayOfMonth'],
                                 month=scheduler['month'],
                                 dayOfWeek=scheduler['dayOfWeek'])
      elif scheduler['type'] == 'TryJobSubversion':
        pools = BuildersPools(s_name)
        pools[s_name].extend(scheduler['builders'])
        instance = try_job_svn.TryJobSubversion(
            name=s_name,
            svn_url=TRY_SVN_BASEURL,
            last_good_urls={'skia': None},
            code_review_sites={'skia': config_private.CODE_REVIEW_SITE},
            pools=pools)
      elif scheduler['type'] == 'TryJobRietveld':
        pools = BuildersPools(s_name)
        pools[s_name].extend(scheduler['builders'])
        instance = try_job_rietveld.TryJobRietveld(
            name=s_name,
            pools=pools,
            last_good_urls={'skia': None},
            code_review_sites={'skia': config_private.CODE_REVIEW_SITE},
            project='skia')
      else:
        raise ValueError(
            'The scheduler type is unrecognized %s' % scheduler['type'])
      scheduler['instance'] = instance
      c['schedulers'].append(instance)

      # Find the union of all sets of subdirectories the builders care about.
      if scheduler.has_key('branch'):
        if not scheduler['branch'] in all_subdirs:
          all_subdirs.append(scheduler['branch'])
      if scheduler.has_key('branches'):
        for branch in scheduler['branches']:
          if not branch in all_subdirs:
            all_subdirs.append(branch)

    # Export the set to be used externally, making sure that it hasn't already
    # been defined.
    # pylint: disable=W0601
    global skia_all_subdirs
    try:
      if skia_all_subdirs:
        raise Exception('skia_all_subdirs has already been defined!')
    except NameError:
      skia_all_subdirs = all_subdirs


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


def MakeSchedulerName(builder_base_name):
  return MakeBuilderName(builder_base_name, 'Scheduler')


def _MakeBuilderSet(helper, builder_base_name, gm_image_subdir,
                    perf_output_basedir=None, extra_branches=None,
                    factory_type=None, do_debug=True, do_release=True,
                    do_bench=True, try_schedulers=None, **kwargs):
  """ Creates a trio of builders for a given platform:
  1. Debug mode builder which runs all steps
  2. Release mode builder which runs all steps EXCEPT benchmarks
  3. Release mode builder which runs ONLY benchmarks.
  """
  B = helper.Builder
  F = helper.Factory

  if not extra_branches:
    extra_branches = []
  subdirs_to_checkout = set(extra_branches)
  if gm_image_subdir:
    gm_image_branch = 'gm-expected/%s' % gm_image_subdir
    subdirs_to_checkout.add(gm_image_branch)

  if try_schedulers:
    scheduler_name = '|'.join(try_schedulers)
    builder_base_name = builder_base_name + TRYBOT_NAME_SUFFIX
  else:
    scheduler_name = MakeSchedulerName(builder_base_name)
    branches = list(subdirs_to_checkout.union(SKIA_PRIMARY_SUBDIRS))
    helper.AnyBranchScheduler(scheduler_name, branches=branches)

  if do_debug:
    debug_builder_name = MakeDebugBuilderName(builder_base_name)
    B(debug_builder_name, 'f_%s' % debug_builder_name,
        scheduler=scheduler_name)
    F('f_%s' % debug_builder_name, factory_type(
        builder_name=debug_builder_name,
        other_subdirs=subdirs_to_checkout,
        configuration=skia_factory.CONFIG_DEBUG,
        gm_image_subdir=gm_image_subdir,
        do_patch_step=(try_schedulers is not None),
        perf_output_basedir=None,
        **kwargs
        ).Build())

  if do_release:
    no_perf_builder_name = MakeReleaseBuilderName(builder_base_name)
    B(no_perf_builder_name, 'f_%s' % no_perf_builder_name,
        scheduler=scheduler_name)
    F('f_%s' % no_perf_builder_name,  factory_type(
        builder_name=no_perf_builder_name,
        other_subdirs=subdirs_to_checkout,
        configuration=skia_factory.CONFIG_RELEASE,
        gm_image_subdir=gm_image_subdir,
        do_patch_step=(try_schedulers is not None),
        perf_output_basedir=None,
        **kwargs
        ).BuildNoPerf())

  if do_bench:
    perf_builder_name = MakeBenchBuilderName(builder_base_name)
    B(perf_builder_name, 'f_%s' % perf_builder_name,
        scheduler=scheduler_name)
    F('f_%s' % perf_builder_name, factory_type(
        builder_name=perf_builder_name,
        other_subdirs=subdirs_to_checkout,
        configuration=skia_factory.CONFIG_RELEASE,
        gm_image_subdir=gm_image_subdir,
        do_patch_step=(try_schedulers is not None),
        perf_output_basedir=perf_output_basedir,
        **kwargs        
        ).BuildPerfOnly())


def _MakeBuilderAndMaybeTrybotSet(do_trybots=True, **kwargs):
  _MakeBuilderSet(try_schedulers=None, **kwargs)
  if do_trybots:
    _MakeBuilderSet(try_schedulers=TRY_SCHEDULERS, **kwargs)


def MakeBuilderSet(**kwargs):
  _MakeBuilderAndMaybeTrybotSet(factory_type=skia_factory.SkiaFactory, **kwargs)


def MakeHousekeeperBuilderSet(helper, do_trybots, do_upload_results):
  B = helper.Builder
  F = helper.Factory

  builder_factory_scheduler = [
    # The Percommit housekeeper
    ('Skia_PerCommit_House_Keeping',
     housekeeping_percommit_factory.HouseKeepingPerCommitFactory,
     'skia_rel'),
    # The Periodic housekeeper
    ('Skia_Periodic_House_Keeping',
     housekeeping_periodic_factory.HouseKeepingPeriodicFactory,
     'skia_periodic'),
  ]
  if do_trybots:
    # Add the corresponding trybot builders to the above list.
    builder_factory_scheduler.extend([
        (builder + TRYBOT_NAME_SUFFIX, factory, TRY_SCHEDULERS_STR)
        for (builder, factory, _scheduler) in builder_factory_scheduler])

  for (builder_name, factory, scheduler) in builder_factory_scheduler:
    B(builder_name, 'f_%s' % builder_name, scheduler=scheduler)
    F('f_%s' % builder_name,
      factory(
        do_upload_results=do_upload_results,
        target_platform=skia_factory.TARGET_PLATFORM_LINUX,
        builder_name=builder_name,
        do_patch_step=(scheduler == TRY_SCHEDULERS_STR),
        use_skp_playback_framework=True,
      ).Build())


def MakeAndroidBuilderSet(extra_branches=None, **kwargs):
  if not extra_branches:
    extra_branches = []
  extra_branches.append('android')
  _MakeBuilderAndMaybeTrybotSet(factory_type=android_factory.AndroidFactory,
                                extra_branches=extra_branches,
                                **kwargs)


def MakeChromeOSBuilderSet(**kwargs):
  _MakeBuilderAndMaybeTrybotSet(factory_type=chromeos_factory.ChromeOSFactory,
                                **kwargs)


def MakeIOSBuilderSet(**kwargs):
  _MakeBuilderAndMaybeTrybotSet(factory_type=ios_factory.iOSFactory, **kwargs)


def CanMergeBuildRequests(req1, req2):
  """ Determine whether or not two BuildRequests can be merged. Note that the
  call to buildbot.sourcestamp.SourceStamp.canBeMergedWith() is conspicuously
  missing. This is because that method verifies that:
  1. req1.source.repository == req2.source.repository
  2. req1.source.project == req2.source.project
  3. req1.source.branch == req2.source.branch
  4. req1.patch == None and req2.patch = None
  5. (req1.source.changes and req2.source.changes) or \
     (not req1.source.changes and not req2.source.changes and \
      req1.source.revision == req2.source.revision) 

  Of the above, we want 1, 2, and 5.
  Instead of 3, we want to merge requests if both branches are the same or both
  are listed in skia_all_subdirs. So we duplicate most of that logic here.
  Instead of 4, we want to make sure that neither request is a Trybot request.
  """
  # Verify that the repositories are the same (#1 above).
  if req1.source.repository != req2.source.repository:
    return False

  # Verify that the projects are the same (#2 above).
  if req1.source.project != req2.source.project:
    return False

  # Verify that the branches are the same OR that both requests are from
  # branches we deem mergeable (modification of #3 above).
  if req1.source.branch != req2.source.branch:
    if req1.source.branch not in skia_all_subdirs or \
       req2.source.branch not in skia_all_subdirs:
      return False

  # If either is a try request, don't merge (#4 above).
  if IsTrybot(req1.buildername) or IsTrybot(req2.buildername):
    return False

  # Verify that either: both requests are associated with changes OR neither
  # request is associated with a change but the revisions match (#5 above).
  if req1.source.changes and not req2.source.changes:
    return False
  if not req1.source.changes and req2.source.changes:
    return False
  if not (req1.source.changes and req2.source.changes):
    if req1.source.revision != req2.source.revision:
      return False
  
  return True
