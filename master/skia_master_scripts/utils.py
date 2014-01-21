# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Miscellaneous utilities needed by the Skia buildbot master."""


import difflib
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

import builder_name_schema
import config_private
import os
import skia_vars
import subprocess


GATEKEEPER_NAME = 'GateKeeper'

TRY_SCHEDULER_SVN = 'skia_try_svn'
TRY_SCHEDULER_RIETVELD = 'skia_try_rietveld'
TRY_SCHEDULERS = [TRY_SCHEDULER_SVN, TRY_SCHEDULER_RIETVELD]
TRY_SCHEDULERS_STR = '|'.join(TRY_SCHEDULERS)


def StringDiff(expected, actual):
  """ Returns the diff between two multiline strings, as a multiline string."""
  return ''.join(difflib.unified_diff(expected.splitlines(1),
                                      actual.splitlines(1)))


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


def FixGitSvnEmail(addr):
  """ Git-svn tacks a git-svn-id onto email addresses. This function removes it.

  For example, "skia.buildbots@gmail.com@2bbb7eff-a529-9590-31e7-b0007b416f81"
  becomes, "skia.buildbots@gmail.com". Addresses containing a single '@' will be
  unchanged.
  """
  return '@'.join(addr.split('@')[:2])


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


def CapWordsToUnderscores(string):
  """ Converts a string containing capitalized words to one in which all
  characters are lowercase and words are separated by underscores.

  Examples:
    'NexusS' becomes 'nexus_s'
    'Nexus10' becomes 'nexus_10'

  string: string; string to manipulate.
  """
  name_parts = []
  for part in re.split('(\d+)', string):
    if re.match('(\d+)', part):
      name_parts.append(part)
    else:
      name_parts.extend(re.findall('[A-Z][a-z]*', part))
  return '_'.join([part.lower() for part in name_parts])


def UnderscoresToCapWords(string):
  """ Converts a string lowercase words separated by underscores to one in which
  words are capitalized and not separated by underscores.

  Examples:
    'nexus_s' becomes 'NexusS'
    'nexus_10' becomes 'Nexus10'

  string: string; string to manipulate.
  """
  name_parts = string.split('_')
  return ''.join([part.title() for part in name_parts])


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
    # Override the category with the first two parts of the builder name.
    name_parts = name.split(builder_name_schema.BUILDER_NAME_SEP)
    category = name_parts[0]
    subcategory = name_parts[1] if len(name_parts) > 1 else 'default'
    old_category = self._defaults.get('category')
    self._defaults['category'] = '|'.join((category, subcategory))
    super(SkiaHelper, self).Builder(name=name, factory=factory,
                                    gatekeeper=gatekeeper, scheduler=scheduler,
                                    builddir=builddir, auto_reboot=auto_reboot,
                                    notify_on_missing=notify_on_missing)
    self._defaults['category'] = old_category

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
    c['builders'].sort(key=lambda builder: builder['name'])
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
            builders=scheduler['builders'],
            branch=skia_vars.GetGlobalVariable('master_branch_name'),
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


# Redefining name from outer scope (os) pylint:disable=W0621
def _MakeBuilder(helper, role, os, model, gpu, configuration, arch,
                 factory_type, extra_config=None, perf_output_basedir=None,
                 extra_repos=None, is_trybot=False, **kwargs):
  """ Creates a builder and scheduler. """
  B = helper.Builder
  F = helper.Factory

  if not extra_repos:
    extra_repos = []
  repos_to_checkout = set(extra_repos)

  builder_name = builder_name_schema.MakeBuilderName(
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
    scheduler_name = 'skia_rel'

  B(builder_name, 'f_%s' % builder_name, scheduler=scheduler_name)
  F('f_%s' % builder_name, factory_type(
      builder_name=builder_name,
      other_repos=repos_to_checkout,
      configuration=configuration,
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

# Redefining name from outer scope (os) pylint:disable=W0621
def _MakeCompileBuilder(helper, scheduler, os, compiler, configuration,
                        target_arch, factory_type, is_trybot,
                        extra_config=None, **kwargs):
  builder_name = builder_name_schema.MakeBuilderName(
      role=builder_name_schema.BUILDER_ROLE_BUILD,
      os=os,
      compiler=compiler,
      configuration=configuration,
      target_arch=target_arch,
      extra_config=extra_config,
      is_trybot=is_trybot)

  helper.Builder(builder_name, 'f_%s' % builder_name,
                 # Do not add gatekeeper for trybots.
                 gatekeeper=None if is_trybot else GATEKEEPER_NAME,
                 scheduler=scheduler)
  helper.Factory('f_%s' % builder_name, factory_type(
      builder_name=builder_name,
      do_patch_step=is_trybot,
      configuration=configuration,
      **kwargs
      ).Build(role=builder_name_schema.BUILDER_ROLE_BUILD))
  return builder_name


def MakeCompileBuilderSet(scheduler, do_trybots=True, **kwargs):
  if do_trybots:
    _MakeCompileBuilder(scheduler=TRY_SCHEDULERS_STR, is_trybot=True, **kwargs)
  _MakeCompileBuilder(scheduler=scheduler, is_trybot=False, **kwargs)


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
  are listed in SKIA_PRIMARY_SUBDIRS. So we duplicate most of that logic here.
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
    if req1.source.branch not in SKIA_PRIMARY_SUBDIRS or \
       req2.source.branch not in SKIA_PRIMARY_SUBDIRS:
      return False

  # If either is a try request, don't merge (#4 above).
  if (builder_name_schema.IsTrybot(req1.buildername) or
      builder_name_schema.IsTrybot(req2.buildername)):
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


def get_current_revision():
  """Obtain the checked-out buildbot code revision."""
  checkout_dir = os.path.join(os.path.dirname(__file__), os.pardir, os.pardir)
  if os.path.isdir(os.path.join(checkout_dir, '.git')):
    return subprocess.check_output(['git', 'rev-parse', 'HEAD']).strip()
  elif os.path.isdir(os.path.join(checkout_dir, '.svn')):
    return subprocess.check_output(['svnversion', '.']).strip()
  raise Exception('Unable to determine version control system.')
