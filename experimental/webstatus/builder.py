# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Skia's overrides for buildbot.status.web.builder """


from buildbot.status.web import builder
from buildbot.status.web.base import getAndCheckProperties, path_to_builder
from twisted.python import log
from twisted.web import html
from twisted.web.util import Redirect

import re


def force(self, req, auth_ok=False):
    name = req.args.get("username", ["<unknown>"])[0]
    reason = req.args.get("comments", ["<no reason specified>"])[0]
    branch = req.args.get("branch", [""])[0]
    revision = req.args.get("revision", [""])[0]
    repository = req.args.get("repository", [""])[0]
    project = req.args.get("project", [""])[0]

    log.msg("web forcebuild of builder '%s', branch='%s', revision='%s',"
            " repository='%s', project='%s' by user '%s'" % (
            self.builder_status.getName(), branch, revision, repository,
            project, name))

    # check if this is allowed
    if not auth_ok:
        if not self.getAuthz(req).actionAllowed('forceBuild', req, self.builder_status):
            log.msg("..but not authorized")
            return Redirect(path_to_authfail(req))

    # keep weird stuff out of the branch revision, and property strings.
    # TODO: centralize this somewhere.
    if not re.match(r'^[\w.+/~-]*$', branch):
        log.msg("bad branch '%s'" % branch)
        return Redirect(path_to_builder(req, self.builder_status))
    if not re.match(r'^[ \w\.\-\/]*$', revision):
        log.msg("bad revision '%s'" % revision)
        return Redirect(path_to_builder(req, self.builder_status))
    properties = getAndCheckProperties(req)
    if properties is None:
        return Redirect(path_to_builder(req, self.builder_status))
    if not branch:
        branch = None
    if not revision:
        revision = None

    master = self.getBuildmaster(req)
    d = master.db.sourcestamps.addSourceStamp(branch=branch,
            revision=revision, project=project, repository=repository)
    def make_buildset(ssid):
        r = ("The web-page 'force build' button was pressed by '%s': %s\n"
             % (html.escape(name), html.escape(reason)))
        for s in master.allSchedulers():
          if self.builder_status.getName() in s.builderNames:
            return s.addBuildsetForSourceStamp(ssid=ssid, reason=r,
                                               properties=properties.asDict())
    d.addCallback(make_buildset)
    d.addErrback(log.err, "(ignored) while trying to force build")
    # send the user back to the builder page
    return Redirect(path_to_builder(req, self.builder_status))
builder.StatusResourceBuilder.force = force