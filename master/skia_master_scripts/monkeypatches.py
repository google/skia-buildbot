# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Monkeypatches to override upstream code. """


from buildbot.process.properties import Properties
from buildbot.schedulers.trysched import BadJobfile
from buildbot.status.builder import EXCEPTION
from buildbot.status.web import base as webstatus_base
from buildbot.status.web.status_json import BuilderJsonResource
from buildbot.status.web.status_json import BuildersJsonResource
from buildbot.status.web.status_json import ChangeSourcesJsonResource
from buildbot.status.web.status_json import JsonResource
from buildbot.status.web.status_json import JsonStatusResource
from buildbot.status.web.status_json import MetricsJsonResource
from buildbot.status.web.status_json import ProjectJsonResource
from buildbot.status.web.status_json import SlavesJsonResource
from master import try_job_base
from master import try_job_rietveld
from master import try_job_svn
from master.try_job_base import text_to_dict
from twisted.internet import defer
from twisted.python import log
from twisted.web import server

import config_private
import json
import utils


################################################################################
############################# Trybot Monkeypatches #############################
################################################################################


@defer.deferredGenerator
def SubmitTryJobChanges(self, changes):
  """ Override of SVNPoller.submit_changes:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/scripts/master/try_job_svn.py?view=markup

  We modify it so that the patch file url is added to the build properties.
  This allows the slave to download the patch directly rather than receiving
  it from the master.
  """
  for chdict in changes:
    # pylint: disable=E1101
    parsed = self.parent.parse_options(text_to_dict(chdict['comments']))

    # 'fix' revision.
    # LKGR must be known before creating the change object.
    wfd = defer.waitForDeferred(self.parent.get_lkgr(parsed))
    yield wfd
    wfd.getResult()

    wfd = defer.waitForDeferred(self.master.addChange(
      author=','.join(parsed['email']),
      revision=parsed['revision'],
      comments='',
      properties={'patch_file_url': chdict['repository'] + '/' + \
                      chdict['files'][0]}))
    yield wfd
    change = wfd.getResult()

    self.parent.addChangeInner(chdict['files'], parsed, change.number)

try_job_svn.SVNPoller.submit_changes = SubmitTryJobChanges


def TryJobCreateBuildset(self, ssid, parsed_job):
  """ Override of TryJobBase.create_buildset:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/scripts/master/try_job_base.py?view=markup

  We modify it to verify that the requested builders are in the builder pool for
  this try scheduler. This prevents try requests from running on builders which
  are not registered as trybots. This apparently isn't a problem for Chromium
  since they use a separate try master.
  """
  log.msg('Creating try job(s) %s' % ssid)
  result = None
  for builder in parsed_job['bot']:
    if builder in self.pools[self.name]:
      result = self.addBuildsetForSourceStamp(ssid=ssid,
          reason=parsed_job['name'],
          external_idstring=parsed_job['name'],
          builderNames=[builder],
          properties=self.get_props(builder, parsed_job))
    else:
      log.msg('Scheduler: %s rejecting try job for builder: %s not in %s' % (
                  self.name,
                  builder,
                  self.pools[self.name]))
  return result

try_job_base.TryJobBase.create_buildset = TryJobCreateBuildset


def HtmlResourceRender(self, request):
  """ Override of buildbot.status.web.base.HtmlResource.render:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/third_party/buildbot_8_4p1/buildbot/status/web/base.py?view=markup

  We modify it to pass additional variables on to the web status pages, and
  remove the "if False" section.
  """
  # tell the WebStatus about the HTTPChannel that got opened, so they
  # can close it if we get reconfigured and the WebStatus goes away.
  # They keep a weakref to this, since chances are good that it will be
  # closed by the browser or by us before we get reconfigured. See
  # ticket #102 for details.
  if hasattr(request, "channel"):
    # web.distrib.Request has no .channel
    request.site.buildbot_service.registerChannel(request.channel)

  ctx = self.getContext(request)

  ############################## Added by borenet ##############################
  status = self.getStatus(request)
  ctx['all_builders'] = status.getBuilderNames()
  ctx['skia_repo'] = config_private.SKIA_SVN_BASEURL
  ctx['try_repo'] = config_private.TRY_SVN_BASEURL
  ctx['internal_port'] = config_private.Master.Skia.master_port
  ctx['external_port'] = config_private.Master.Skia.master_port_alt
  ctx['title_url'] = config_private.Master.Skia.project_url
  ##############################################################################

  d = defer.maybeDeferred(lambda : self.content(request, ctx))
  def handle(data):
    if isinstance(data, unicode):
      data = data.encode("utf-8")
    request.setHeader("content-type", self.contentType)
    if request.method == "HEAD":
      request.setHeader("content-length", len(data))
      return ''
    return data
  d.addCallback(handle)
  def ok(data):
    request.write(data)
    request.finish()
  def fail(f):
    request.processingFailed(f)
    return None # processingFailed will log this for us
  d.addCallbacks(ok, fail)
  return server.NOT_DONE_YET

webstatus_base.HtmlResource.render = HtmlResourceRender


class TryBuildersJsonResource(JsonResource):
  """ Clone of buildbot.status.web.status_json.BuildersJsonResource:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/third_party/buildbot_8_4p1/buildbot/status/web/status_json.py?view=markup

  We add filtering to display only the try builders.
  """
  help = """List of all the try builders defined on a master."""
  pageTitle = 'Builders'

  def __init__(self, status):
    JsonResource.__init__(self, status)
    for builder_name in self.status.getBuilderNames():
      if builder_name.endswith(utils.TRYBOT_NAME_SUFFIX):
        self.putChild(builder_name,
                      BuilderJsonResource(status,
                                          status.getBuilder(builder_name)))


def JsonStatusResourceInit(self, status):
  """ Override of buildbot.status.web.status_json.JsonStatusResource.__init__:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/third_party/buildbot_8_4p1/buildbot/status/web/status_json.py?view=markup

  We add a line which adds a trybots status page.
  """
  JsonResource.__init__(self, status)
  self.level = 1
  self.putChild('builders', BuildersJsonResource(status))
  self.putChild('change_sources', ChangeSourcesJsonResource(status))
  self.putChild('project', ProjectJsonResource(status))
  self.putChild('slaves', SlavesJsonResource(status))
  self.putChild('metrics', MetricsJsonResource(status))

  ############################## Added by borenet ##############################
  # Added to address: https://code.google.com/p/skia/issues/detail?id=1134
  self.putChild('trybots', TryBuildersJsonResource(status))
  ##############################################################################

  # This needs to be called before the first HelpResource().body call.
  self.hackExamples()

JsonStatusResource.__init__ = JsonStatusResourceInit


@defer.deferredGenerator
def TryJobRietveldSubmitJobs(self, jobs):
  """ Override of master.try_job_rietveld.TryJobRietveld.SubmitJobs:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/scripts/master/try_job_rietveld.py?view=markup

  We modify it to include "baseurl" as a build property.
  """
  log.msg('TryJobRietveld.SubmitJobs: %s' % json.dumps(jobs, indent=2))
  for job in jobs:
    try:
      # Gate the try job on the user that requested the job, not the one that
      # authored the CL.
      # pylint: disable=W0212
      if not self._valid_users.contains(job['requester']):
        raise BadJobfile(
            'TryJobRietveld rejecting job from %s' % job['requester'])

      if job['email'] != job['requester']:
        # Note the fact the try job was requested by someone else in the
        # 'reason'.
        job['reason'] = job.get('reason') or ''
        if job['reason']:
          job['reason'] += '; '
        job['reason'] += "This CL was triggered by %s" % job['requester']

      options = {
          'bot': {job['builder']: job['tests']},
          'email': [job['email']],
          'project': [self._project],
          'try_job_key': job['key'],
      }
      # Transform some properties as is expected by parse_options().
      for key in (
          ########################## Added by borenet ##########################
          'baseurl',
          ######################################################################
          'name', 'user', 'root', 'reason', 'clobber', 'patchset',
          'issue', 'requester', 'revision'):
        options[key] = [job[key]]

      # Now cleanup the job dictionary and submit it.
      cleaned_job = self.parse_options(options)

      wfd = defer.waitForDeferred(self.get_lkgr(cleaned_job))
      yield wfd
      wfd.getResult()

      wfd = defer.waitForDeferred(self.master.addChange(
          author=','.join(cleaned_job['email']),
          # TODO(maruel): Get patchset properties to get the list of files.
          # files=[],
          revision=cleaned_job['revision'],
          comments=''))
      yield wfd
      changeids = [wfd.getResult().number]

      wfd = defer.waitForDeferred(self.SubmitJob(cleaned_job, changeids))
      yield wfd
      wfd.getResult()
    except BadJobfile, e:
      # We need to mark it as failed otherwise it'll stay in the pending
      # state. Simulate a buildFinished event on the build.
      if not job.get('key'):
        log.err(
            'Got %s for issue %s but not key, not updating Rietveld' %
            (e, job.get('issue')))
        continue
      log.err(
          'Got %s for issue %s, updating Rietveld' % (e, job.get('issue')))
      for service in self.master.services:
        if service.__class__.__name__ == 'TryServerHttpStatusPush':
          # pylint: disable=W0212
          build = {
            'properties': [
              ('buildername', job.get('builder'), None),
              ('buildnumber', -1, None),
              ('issue', job['issue'], None),
              ('patchset', job['patchset'], None),
              ('project', self._project, None),
              ('revision', '', None),
              ('slavename', '', None),
              ('try_job_key', job['key'], None),
            ],
            'reason': job.get('reason', ''),
            # Use EXCEPTION until SKIPPED results in a non-green try job
            # results on Rietveld.
            'results': EXCEPTION,
          }
          service.push('buildFinished', build=build)
          break

try_job_rietveld.TryJobRietveld.SubmitJobs = TryJobRietveldSubmitJobs

def TryJobBaseGetProps(self, builder, options):
  """ Override of try_job_base.TryJobBase.get_props:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/scripts/master/try_job_base.py?view=markup

  We modify it to add "baseurl".
  """
  keys = (
############################### Added by borenet ###############################
    'baseurl',
################################################################################
    'clobber',
    'issue',
    'patchset',
    'requester',
    'rietveld',
    'root',
    'try_job_key',
  )
  # All these settings have no meaning when False or not set, so don't set
  # them in that case.
  properties = dict((i, options[i]) for i in keys if options.get(i))
  properties['testfilter'] = options['bot'].get(builder, None)
  # pylint: disable=W0212
  props = Properties()
  props.updateFromProperties(self.properties)
  props.update(properties, self._PROPERTY_SOURCE)
  return props

try_job_base.TryJobBase.get_props = TryJobBaseGetProps