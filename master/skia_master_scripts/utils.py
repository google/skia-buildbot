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
from master import try_job_svn
from master import try_job_rietveld
from master.builders_pools import BuildersPools
from oauth2client.client import SignedJwtAssertionCredentials

import config_private


BUILDER_NAME_SEP = '-'

# Patterns for creating builder names, based on the role of the builder.
# TODO(borenet): Extract these into a separate file (JSON?) so that they can be
# read by other users.
BUILDER_ROLE_COMPILE = 'Build'
BUILDER_ROLE_PERF = 'Perf'
BUILDER_ROLE_TEST = 'Test'
BUILDER_ROLE_HOUSEKEEPER = 'Housekeeper'
BUILDER_NAME_DEFAULT_ATTRS = ['os', 'model', 'gpu', 'arch', 'configuration']
BUILDER_NAME_SCHEMA = {
  BUILDER_ROLE_COMPILE: ['os', 'compiler', 'target_arch', 'configuration'],
  BUILDER_ROLE_TEST: BUILDER_NAME_DEFAULT_ATTRS,
  BUILDER_ROLE_PERF: BUILDER_NAME_DEFAULT_ATTRS,
  BUILDER_ROLE_HOUSEKEEPER: ['frequency'],
}

CATEGORY_BUILD = ' Build'
TRYBOT_NAME_SUFFIX = 'Trybot'
TRY_SCHEDULER_SVN = 'skia_try_svn'
TRY_SCHEDULER_RIETVELD = 'skia_try_rietveld'
TRY_SCHEDULERS = [TRY_SCHEDULER_SVN, TRY_SCHEDULER_RIETVELD]
TRY_SCHEDULERS_STR = '|'.join(TRY_SCHEDULERS)


def IsTrybot(builder_name):
  return builder_name.endswith(TRYBOT_NAME_SUFFIX)


def _IndentStr(indent):
  return '    ' * (indent + 1)


def ToString(obj, indent=0):
  """ Returns a string representation of the given object. This differs from the
  built-in string function in that it does not give memory locations.

  obj: the object to print.
  indent: integer; the current indent level.
  """
  if isinstance(obj, list):
    return _ListToString(obj, indent)
  elif isinstance(obj, dict):
    return _DictToString(obj, indent)
  elif isinstance(obj, tuple):
    return _ListToString(obj, indent)
  elif isinstance(obj, str):
    return '\'%s\'' % obj
  elif obj is None:
    return 'None'
  else:
    return '<Object>'


def _ListToString(list_var, indent):
  if not list_var:
    return '[]'
  indent_str = _IndentStr(indent)
  val = '[\n'
  indent += 1
  val += ''.join(['%s%s,\n' % (indent_str, ToString(elem, indent)) \
                  for elem in list_var])
  indent -= 1
  indent_str = _IndentStr(indent - 1)
  val += indent_str + ']'
  return val


def _DictToString(d, indent):
  if not d:
    return '{}'
  indent_str = _IndentStr(indent)
  val = '{\n'
  indent += 1
  val += ''.join(['%s%s: %s,\n' % (indent_str, ToString(k, indent),
                                   ToString(d[k], indent)) \
                  for k in sorted(d.keys())])
  indent -= 1
  indent_str = _IndentStr(indent - 1)
  val += indent_str + '}'
  return val


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
SKIA_PRIMARY_SUBDIRS = ['buildbot', 'trunk']


# Skip buildbot runs of a CL if its commit log message contains the following
# substring.
SKIP_BUILDBOT_SUBSTRING = '(SkipBuildbotRuns)'

# If the below regex is found in a CL's commit log message, only run the
# builders specified therein.
RUN_BUILDERS_REGEX = '\(RunBuilders:(.+)\)'
RUN_BUILDERS_RE_COMPILED = re.compile(RUN_BUILDERS_REGEX)


def AndroidModelToDevice(android_model):
  """ Converts Android model names to device names which android_setup.sh will
  like.

  Examples:
    'NexusS' becomes 'nexus_s'
    'Nexus10' becomes 'nexus_10'

  android_model: string; model name for an Android device.
  """
  name_parts = []
  for part in re.split('(\d+)', android_model):
    if re.match('(\d+)', part):
      name_parts.append(part)
    else:
      name_parts.extend(re.findall('[A-Z][a-z]*', part))
  return '_'.join([part.lower() for part in name_parts])


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


def MakeBuilderName(role, extra_config=None, is_trybot=False, **kwargs):
  schema = BUILDER_NAME_SCHEMA.get(role)
  if not schema:
    raise ValueError('%s is not a recognized role.' % role)
  for k, v in kwargs.iteritems():
    if BUILDER_NAME_SEP in v:
      raise ValueError('%s not allowed in %s.' % (v, BUILDER_NAME_SEP))
    if not k in schema:
      raise ValueError('Schema does not contain "%s": %s' %(k, schema))
  if extra_config and BUILDER_NAME_SEP in extra_config:
    raise ValueError('%s not allowed in %s.' % (extra_config,
                                                BUILDER_NAME_SEP))
  name_parts = [role]
  name_parts.extend([kwargs[attribute] for attribute in schema])
  if extra_config:
    name_parts.append(extra_config)
  if is_trybot:
    name_parts.append(TRYBOT_NAME_SUFFIX)
  print BUILDER_NAME_SEP.join(name_parts)
  return BUILDER_NAME_SEP.join(name_parts)


def _MakeBuilder(helper, role, os, model, gpu, configuration, arch,
                 gm_image_subdir, factory_type, extra_config=None,
                 perf_output_basedir=None, extra_branches=None, is_trybot=False,
                 **kwargs):
  """ Creates a builder and scheduler. """
  B = helper.Builder
  F = helper.Factory

  if not extra_branches:
    extra_branches = []
  subdirs_to_checkout = set(extra_branches)
  if gm_image_subdir:
    gm_image_branch = 'gm-expected/%s' % gm_image_subdir
    subdirs_to_checkout.add(gm_image_branch)

  builder_name = MakeBuilderName(
      role=role,
      os=os,
      model=model,
      gpu=gpu,
      configuration=configuration,
      arch=arch,
      extra_config=extra_config,
      is_trybot=is_trybot)

  if is_trybot:
    scheduler_name = TRY_SCHEDULERS_STR
  else:
    scheduler_name = builder_name + BUILDER_NAME_SEP + 'Scheduler'
    branches = list(subdirs_to_checkout.union(SKIA_PRIMARY_SUBDIRS))
    helper.AnyBranchScheduler(scheduler_name, branches=branches)

  B(builder_name, 'f_%s' % builder_name, scheduler=scheduler_name)
  F('f_%s' % builder_name, factory_type(
      builder_name=builder_name,
      other_subdirs=subdirs_to_checkout,
      configuration=configuration,
      gm_image_subdir=gm_image_subdir,
      do_patch_step=is_trybot,
      perf_output_basedir=perf_output_basedir,
      **kwargs
      ).Build(role=role))


def _MakeBuilderAndMaybeTrybotSet(do_trybots=True, **kwargs):
  _MakeBuilder(**kwargs)
  if do_trybots:
    _MakeBuilder(is_trybot=True, **kwargs)


def MakeBuilderSet(**kwargs):
  _MakeBuilderAndMaybeTrybotSet(**kwargs)


def _MakeCompileBuilder(helper, scheduler, os, compiler, configuration,
                        target_arch, factory_type, is_trybot,
                        extra_config=None, **kwargs):
  builder_name = MakeBuilderName(role=BUILDER_ROLE_COMPILE,
                                 os=os,
                                 compiler=compiler,
                                 configuration=configuration,
                                 target_arch=target_arch,
                                 extra_config=extra_config,
                                 is_trybot=is_trybot)
  helper.Builder(builder_name, 'f_%s' % builder_name,
                 # Do not add gatekeeper for trybots.
                 gatekeeper='GateKeeper' if is_trybot else None,
                 scheduler=scheduler)
  helper.Factory('f_%s' % builder_name, factory_type(
      builder_name=builder_name,
      do_patch_step=is_trybot,
      configuration=configuration,
      **kwargs
      ).Build(role=BUILDER_ROLE_COMPILE))
  return builder_name


def MakeCompileBuilderSet(scheduler, do_trybots=True, **kwargs):
  if do_trybots:
    _MakeCompileBuilder(scheduler=scheduler, is_trybot=True, **kwargs)
  _MakeCompileBuilder(scheduler=TRY_SCHEDULERS_STR, is_trybot=False, **kwargs)


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
