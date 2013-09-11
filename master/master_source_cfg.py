# Copyright (c) 2011 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from config_private import SKIA_GIT_URL
from master import chromium_git_poller_bb8

import skia_vars


def Update(config, active_master, c):
  skia_poller = chromium_git_poller_bb8.ChromiumGitPoller(
      repourl=SKIA_GIT_URL,
      pollInterval=30,
      revlinktmpl=skia_vars.GetGlobalVariable('revlink_tmpl'))
  c['change_source'].append(skia_poller)
