# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

from buildbot.changes import svnpoller

from master import build_utils

def SkiaFileSplitter(path):
  """split_file for Skia."""
  projects = ['trunk']
  return build_utils.SplitPath(projects, path)

def Update(config, active_master, c):
  skia_url = config.Master.skia_url
  skia_revlinktmpl = 'http://code.google.com/p/skia/source/browse?r=%s'

  skia_poller = svnpoller.SVNPoller(svnurl=skia_url,
                                    split_file=SkiaFileSplitter,
                                    pollinterval=30,
                                    revlinktmpl=skia_revlinktmpl)
  c['change_source'].append(skia_poller)
