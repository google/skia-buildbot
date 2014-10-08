# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Miscellaneous utilities needed by the Skia buildbot master."""


import difflib
import httplib2
import json
import os
import re

# requires Google APIs client library for Python; see
# https://code.google.com/p/google-api-python-client/wiki/Installation
from apiclient.discovery import build
from buildbot.scheduler import Dependent
from buildbot.scheduler import Scheduler
from buildbot.schedulers import timed
from buildbot.schedulers.filter import ChangeFilter
from config_private import TRY_SVN_BASEURL
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


def GetListFromEnvVar(name, splitstring=','):
  """ Returns contents of an environment variable, as a list.

  If the environment variable is unset or set to empty-string, this returns
  an empty list.

  name: string; name of the environment variable to read
  splitstring: string with which to split the env var into list items
  """
  unsplit = os.environ.get(name, None)
  if unsplit:
    return unsplit.split(',')
  else:
    return []


def StringDiff(expected, actual):
  """ Returns the diff between two multiline strings, as a multiline string."""
  return ''.join(difflib.unified_diff(expected.splitlines(1),
                                      actual.splitlines(1)))


def ToString(obj):
  """ Returns a string representation of the given object. This differs from the
  built-in string function in that it does not give memory locations.

  obj: the object to print.
  """
  def sanitize(obj):
    if isinstance(obj, list) or isinstance(obj, tuple):
      return [sanitize(sub_obj) for sub_obj in obj]
    elif isinstance(obj, dict):
      rv = {}
      for k, v in obj.iteritems():
        rv[k] = sanitize(v)
      return rv
    elif isinstance(obj, str) or obj is None:
      return obj
    else:
      return '<Object>'
  return json.dumps(sanitize(obj), indent=4, sort_keys=True)


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

  Example:
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

  Example:
    'nexus_10' becomes 'Nexus10'

  string: string; string to manipulate.
  """
  name_parts = string.split('_')
  return ''.join([part.title() for part in name_parts])


# Since we can't modify the existing Helper class, we subclass it here,
# overriding the necessary parts to get things working as we want.
class SkiaHelper(object):
  def __init__(self, defaults):
    self._defaults = defaults
    self._builders = []
    self._factories = {}
    self._schedulers = {}

  def Builder(self, name, factory, gatekeeper=None, scheduler=None,
              builddir=None, auto_reboot=False, notify_on_missing=False):
    # Override the category with the first two parts of the builder name.
    name_parts = name.split(builder_name_schema.BUILDER_NAME_SEP)
    category = name_parts[0]
    subcategory = name_parts[1] if len(name_parts) > 1 else 'default'
    full_category = '|'.join((category, subcategory))
    self._builders.append({'name': name,
                           'factory': factory,
                           'gatekeeper': gatekeeper,
                           'schedulers': scheduler.split('|'),
                           'builddir': builddir,
                           'category': full_category,
                           'auto_reboot': auto_reboot,
                           'notify_on_missing': notify_on_missing})

  def PeriodicScheduler(self, name, minute=0, hour='*', dayOfMonth='*',
                        month='*', dayOfWeek='*'):
    """Helper method for the Periodic scheduler."""
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'PeriodicScheduler',
                              'builders': [],
                              'minute': minute,
                              'hour': hour,
                              'dayOfMonth': dayOfMonth,
                              'month': month,
                              'dayOfWeek': dayOfWeek}

  def Dependent(self, name, parent):
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'Dependent',
                              'parent': parent,
                              'builders': []}

  def Factory(self, name, factory):
    if name in self._factories:
      raise ValueError('Factory %s already exists' % name)
    self._factories[name] = factory

  def Scheduler(self, name, treeStableTimer=60, categories=None):
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'Scheduler',
                              'treeStableTimer': treeStableTimer,
                              'builders': [],
                              'categories': categories}

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
    for builder in self._builders:
      # Update the schedulers with the builder.
      schedulers = builder['schedulers']
      if schedulers:
        for scheduler in schedulers:
          self._schedulers[scheduler]['builders'].append(builder['name'])

      # Construct the category.
      categories = []
      if builder.get('category', None):
        categories.append(builder['category'])
      if builder.get('gatekeeper', None):
        categories.extend(builder['gatekeeper'].split('|'))
      category = '|'.join(categories)

      # Append the builder to the list.
      new_builder = {'name': builder['name'],
                     'factory': self._factories[builder['factory']],
                     'category': category,
                     'auto_reboot': builder['auto_reboot']}
      if builder['builddir']:
        new_builder['builddir'] = builder['builddir']
      c['builders'].append(new_builder)

    c['builders'].sort(key=lambda builder: builder['name'])

    # Process the main schedulers.
    for s_name in self._schedulers:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'Scheduler':
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

        instance = Scheduler(name=s_name,
                             treeStableTimer=scheduler['treeStableTimer'],
                             builderNames=scheduler['builders'],
                             change_filter=skia_change_filter)
        c['schedulers'].append(instance)
        self._schedulers[s_name]['instance'] = instance

    # Process the periodic schedulers.
    for s_name in self._schedulers:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'PeriodicScheduler':
        instance = timed.Nightly(
            name=s_name,
            branch=skia_vars.GetGlobalVariable('master_branch_name'),
            builderNames=scheduler['builders'],
            minute=scheduler['minute'],
            hour=scheduler['hour'],
            dayOfMonth=scheduler['dayOfMonth'],
            month=scheduler['month'],
            dayOfWeek=scheduler['dayOfWeek'])
        c['schedulers'].append(instance)
        self._schedulers[s_name]['instance'] = instance

    # Process the Rietveld-based try schedulers.
    for s_name in self._schedulers:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'TryJobRietveld':
        pools = BuildersPools(s_name)
        pools[s_name].extend(scheduler['builders'])
        instance = try_job_rietveld.TryJobRietveld(
            name=s_name,
            pools=pools,
            last_good_urls={'skia': None},
            code_review_sites={'skia': config_private.CODE_REVIEW_SITE},
            project='skia')
        c['schedulers'].append(instance)
        self._schedulers[s_name]['instance'] = instance

    # Process the svn-based try schedulers.
    for s_name in self._schedulers:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'TryJobSubversion':
        pools = BuildersPools(s_name)
        pools[s_name].extend(scheduler['builders'])
        instance = try_job_svn.TryJobSubversion(
            name=s_name,
            svn_url=TRY_SVN_BASEURL,
            last_good_urls={'skia': None},
            code_review_sites={'skia': config_private.CODE_REVIEW_SITE},
            pools=pools)
        c['schedulers'].append(instance)
        self._schedulers[s_name]['instance'] = instance

    # Process the dependent schedulers.
    for s_name in self._schedulers:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'Dependent':
        instance = Dependent(
            s_name,
            self._schedulers[scheduler['parent']]['instance'],
            scheduler['builders'])
        c['schedulers'].append(instance)
        self._schedulers[s_name]['instance'] = instance


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

  Of the above, we want 1, 2, 3, and 5.
  Instead of 4, we want to make sure that neither request is a Trybot request.
  """
  # Verify that the repositories are the same (#1 above).
  if req1.source.repository != req2.source.repository:
    return False

  # Verify that the projects are the same (#2 above).
  if req1.source.project != req2.source.project:
    return False

  # Verify that the branches are the same (#3 above).
  if req1.source.branch != req2.source.branch:
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
