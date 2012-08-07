# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Miscellaneous utilities needed by the Skia buildbot master."""

from buildbot.scheduler import AnyBranchScheduler
from buildbot.scheduler import Scheduler
from buildbot.util import NotABranch
from master import master_config

def FileBug(summary, description, owner=None, ccs=[], labels=[]):
  """Files a bug to the Skia issue tracker.

  Args:
    summary: a single-line string to use as the issue summary
    description: a multiline string to use as the issue description
    owner: email address of the issue owner (as a string), or None if unknown
    ccs: email addresses (list of strings) to CC on the bug
    labels: labels (list of strings) to apply to the bug
  """
  # TODO: for now, this is a skeletal implementation to aid discussion of
  # https://code.google.com/p/skia/issues/detail?id=726
  # ('buildbot: automatically file bug report when the build goes red')
  tracker_base_url = 'https://code.google.com/p/skia/issues'
  reporter = 'user@domain.com'  # I guess we'll need to set up an account for this purpose
  credentials = None    # presumably we will need credentials to log in as reporter;
                        # note that the credentials should not be included in the source code!
  # Code goes here...

# Branches for which we trigger rebuilds on the primary builders
SKIA_PRIMARY_SUBDIRS = ['android', 'buildbot', 'trunk']

# Since we can't modify the existing Helper class, we subclass it here,
# overriding the necessary parts to get things working as we want.
# Specifically, the Helper class hardcodes each registered scheduler to be
# instantiated as a 'Scheduler,' which aliases 'SingleBranchScheduler.'  We add
# an 'AnyBranchScheduler' method and change the implementation of Update() to
# instantiate the proper type.

# TODO(borenet): modify this code upstream so that we don't need this override.	
# BUG: http://code.google.com/p/skia/issues/detail?id=761
class SkiaHelper(master_config.Helper):
  def AnyBranchScheduler(self, name, branches, treeStableTimer=60,
                         categories=None):
    if name in self._schedulers:
      raise ValueError('Scheduler %s already exist' % name)
    self._schedulers[name] = {'type': 'AnyBranchScheduler',
                              'branches': branches,
                              'treeStableTimer': treeStableTimer,
                              'builders': [],
                              'categories': categories}

  def Update(self, c):
    super(SkiaHelper, self).Update(c)
    for s_name in self._schedulers:
      scheduler = self._schedulers[s_name]
      if scheduler['type'] == 'AnyBranchScheduler':
        instance = AnyBranchScheduler(name=s_name,
                                      branch=NotABranch,
                                      branches=scheduler['branches'],
                                      treeStableTimer=scheduler['treeStableTimer'],
                                      builderNames=scheduler['builders'],
                                      categories=scheduler['categories'])
        scheduler['instance'] = instance
        c['schedulers'].append(instance)
