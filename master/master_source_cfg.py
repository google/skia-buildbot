# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from buildbot.changes import svnpoller
from config_private import SKIA_REVLINKTMPL, SKIA_SVN_BASEURL
from skia_master_scripts import utils


def SkiaFileSplitter(path):
  """split_file for Skia."""
  subdirs = utils.skia_all_subdirs
  for subdir in subdirs:
    if path.startswith(subdir):
      return (subdir, path[len(subdir)+1:])
  return None


def Update(config, active_master, c):
  skia_poller = svnpoller.SVNPoller(svnurl=SKIA_SVN_BASEURL,
                                    split_file=SkiaFileSplitter,
                                    pollinterval=30,
                                    revlinktmpl=SKIA_REVLINKTMPL)
  c['change_source'].append(skia_poller)
