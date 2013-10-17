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
from multi_dependent_scheduler import DependencyChainScheduler
from buildbot.scheduler import Nightly
from buildbot.scheduler import Periodic
from buildbot.scheduler import Scheduler
from buildbot.scheduler import Triggerable
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
import skia_vars


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
class Helper(object):
  def __init__(self):
    self._builders = []
    self._factories = {}
    self._schedulers = {}
    self._schedulers_list = []

  def Builder(self, name, factory, category=None, gatekeeper=None,
              scheduler=None, builddir=None, auto_reboot=True,
              notify_on_missing=False):
    self._builders.append({'name': name,
                           'factory': factory,
                           'gatekeeper': gatekeeper,
                           'schedulers': scheduler.split('|'),
                           'builddir': builddir,
                           'category': category,
                           'auto_reboot': auto_reboot,
                           'notify_on_missing': notify_on_missing})

  def Hourly(self, name, branch, hour='*'):
    """Helper method for the Nightly scheduler."""
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'Nightly',
                              'builders': [],
                              'branch': branch,
                              'hour': hour}
    self._schedulers_list.append(name)

  def Periodic(self, name, periodicBuildTimer):
    """Helper method for the Periodic scheduler."""
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'Periodic',
                              'builders': [],
                              'periodicBuildTimer': periodicBuildTimer}
    self._schedulers_list.append(name)

  def Dependent(self, name, parent):
    if not isinstance(parent, list):
      raise Exception('Parent is a list!')
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'Dependent',
                              'parent': parent,
                              'builders': []}
    self._schedulers_list.append(name)

  def Triggerable(self, name):
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'Triggerable',
                              'builders': []}
    self._schedulers_list.append(name)

  def Factory(self, name, factory):
    if name in self._factories:
      raise ValueError('Factory %s already exists' % name)
    self._factories[name] = factory

  def Scheduler(self, name, branch, treeStableTimer=60, categories=None):
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exists' % name)
    self._schedulers[name] = {'type': 'Scheduler',
                              'branch': branch,
                              'treeStableTimer': treeStableTimer,
                              'builders': [],
                              'categories': categories}
    self._schedulers_list.append(name)

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

    # Process the main schedulers.
    for s_name in self._schedulers_list:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'Scheduler':
        instance = Scheduler(name=s_name,
                             branch=scheduler['branch'],
                             treeStableTimer=scheduler['treeStableTimer'],
                             builderNames=scheduler['builders'],
                             categories=scheduler['categories'])
        scheduler['instance'] = instance
        c['schedulers'].append(instance)

    # Process the dependent schedulers.
    for s_name in self._schedulers_list:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'Dependent':
        if len(scheduler['builders']) > 1:
          raise Exception('DependencyChainScheduler is associated with a single'
                          ' builder.')
        instance = DependencyChainScheduler(name=s_name,
                      dependencies=[self._schedulers[parent]['instance']
                                    for parent in scheduler['parent']],
                      builder_name=scheduler['builders'][0])
        scheduler['instance'] = instance
        c['schedulers'].append(instance)

    # Process the triggerable schedulers.
    for s_name in self._schedulers_list:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'Triggerable':
        c['schedulers'].append(Triggerable(s_name,
                                           scheduler['builders']))

    # Process the periodic schedulers.
    for s_name in self._schedulers_list:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'Periodic':
        c['schedulers'].append(
            Periodic(s_name,
                     periodicBuildTimer=scheduler['periodicBuildTimer'],
                     builderNames=scheduler['builders']))

    # Process the nightly schedulers.
    for s_name in self._schedulers_list:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'Nightly':
        c['schedulers'].append(Nightly(s_name,
                                       branch=scheduler['branch'],
                                       hour=scheduler['hour'],
                                       builderNames=scheduler['builders']))


def _MakeBuilder(helper, role, os, model, gpu, configuration, arch,
                 gm_image_subdir, factory_type, extra_config=None,
                 perf_output_basedir=None, extra_repos=None, is_trybot=False,
                 **kwargs):
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
