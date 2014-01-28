# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""JSON interface which returns the most recent results for each builder."""


import buildbot.status.web.base as base

from buildbot.status.web.status_json import JsonResource
from twisted.python import log


class BuilderStatusesJsonResource(JsonResource):
  """Returns the results of the last completed build for each builder.

  This is essentially a JSON equivalent of the upstream
  horizontal_one_box_per_builder HTML page, as in
  third_party/chromium_buildbot/scripts/master/chromium_status_bb8.py.
  """

  def asDict(self, request):
    builders = request.args.get('builder', self.status.getBuilderNames())
    data = {'builders': []}
    for builder_name in builders:
      try:
        builder_status = self.status.getBuilder(builder_name)
      except KeyError:
        log.msg('status.getBuilder(%r) failed' % builder_name)
        continue
      outcome = base.ITopBox(builder_status).getBox(request).class_
      lastbuild = 'LastBuild'
      if outcome.startswith(lastbuild):
        outcome = outcome[len(lastbuild):]
      data['builders'].append({'outcome': outcome.strip(),
                               'name': builder_name})
    return data