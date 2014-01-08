# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" JSON interface which gives the buildbot master's current code revision. """


from buildbot.status.web.status_json import JsonResource

import utils


class MasterCheckedOutRevisionJsonResource(JsonResource):
  """Revision of the buildbot code on the build master."""
  help = ('Revision of the buildbot code on the build master, which may or may '
          'not be the revision which is actually running, see also '
          'MasterRunningRevisionJsonResource.')
  pageTitle = 'Master Revision'

  def asDict(self, request):
    return {'master_checkedout_revision': utils.get_current_revision()}


class MasterRunningRevisionJsonResource(JsonResource):
  """Revision of the buildbot code which the build master is actually running.
  """
  help = ('Revision of the buildbot code which the build master is actually '
          'running.')
  pageTitle = 'Master Running Revision'

  def __init__(self, running_revision, **kwargs):
    JsonResource.__init__(self, **kwargs)
    self._running_revision = running_revision

  def asDict(self, request):
    return {'master_running_revision': self._running_revision}