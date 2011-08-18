# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from buildbot.changes import svnpoller

from master import build_utils

def ChromeTreeFileSplitter(path):
  """split_file for the 'src' project in the trunk."""

  # Exclude .DEPS.git from triggering builds on chrome.
  if path == 'src/.DEPS.git':
    return None

  # List of projects we are interested in. The project names must exactly
  # match paths in the Subversion repository, relative to the 'path' URL
  # argument. build_utils.SplitPath() will use them as branch names to
  # kick off the Schedulers for different projects.
  projects = ['src']
  return build_utils.SplitPath(projects, path)

def WebkitFileSplitter(path):
  """split_file for webkit.org repository."""
  projects = ['trunk']
  return build_utils.SplitPath(projects, path)


def Update(config, active_master, c):
  # Polls config.Master.trunk_url for changes
  chromium_url = "http://src.chromium.org/viewvc/chrome?view=rev&revision=%s"
  #webkit_url = "http://trac.webkit.org/changeset/%s"
  cr_poller = svnpoller.SVNPoller(svnurl=config.Master.trunk_url,
                                  split_file=ChromeTreeFileSplitter,
                                  pollinterval=30,
                                  revlinktmpl=chromium_url)
  c['change_source'].append(cr_poller)

  #webkit_poller = svnpoller.SVNPoller(svnurl = config.Master.webkit_root_url,
  #                                    split_file=WebkitFileSplitter,
  #                                    pollinterval=30,
  #                                    revlinktmpl=webkit_url)
  #c['change_source'].append(webkit_poller)
