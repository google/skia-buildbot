# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Monkeypatches to override upstream code. """


from buildbot.process.properties import Properties
from buildbot.schedulers.trysched import BadJobfile
from buildbot.status.builder import EXCEPTION, FAILURE
from buildbot.status.web import base as webstatus_base
from buildbot.status.web.status_json import BuilderJsonResource
from buildbot.status.web.status_json import BuildersJsonResource
from buildbot.status.web.status_json import ChangeSourcesJsonResource
from buildbot.status.web.status_json import JsonResource
from buildbot.status.web.status_json import JsonStatusResource
from buildbot.status.web.status_json import MetricsJsonResource
from buildbot.status.web.status_json import ProjectJsonResource
from buildbot.status.web.status_json import SlavesJsonResource
from master import build_utils
from master import chromium_notifier
from master import gatekeeper
from master import try_job_base
from master import try_job_rietveld
from master import try_job_svn
from master.try_job_base import text_to_dict
from twisted.internet import defer
from twisted.python import log
from twisted.web import server
from webstatus import builder_statuses

import builder_name_schema
import config_private
import json
import master_revision
import re
import slave_hosts_cfg
import slaves_cfg
import skia_vars
import utils


# The following users are allowed to run trybots even though they do not have
# accounts in google.com or chromium.org
TRYBOTS_REQUESTER_WHITELIST = [
    'kkinnunen@nvidia.com',
    'ravimist@gmail.com'
]


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
  log.msg('Returning buildset: %s' % result)
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
  all_builders = status.getBuilderNames()
  all_full_category_names = set()
  all_categories = set()
  all_subcategories = set()
  subcategories_by_category = {}
  for builder_name in all_builders:
    category_full = status.getBuilder(builder_name).category or 'default'
    all_full_category_names.add(category_full)
    category_split = category_full.split('|')
    category = category_split[0]
    subcategory = category_split[1] if len(category_split) > 1 else 'default'
    all_categories.add(category)
    all_subcategories.add(subcategory)
    if not subcategories_by_category.get(category):
      subcategories_by_category[category] = []
    if not subcategory in subcategories_by_category[category]:
      subcategories_by_category[category].append(subcategory)

  ctx['tree_status_baseurl'] = \
      skia_vars.GetGlobalVariable('tree_status_baseurl')

  ctx['all_full_category_names'] = sorted(list(all_full_category_names))
  ctx['all_categories'] = sorted(list(all_categories))
  ctx['all_subcategories'] = sorted(list(all_subcategories))
  ctx['subcategories_by_category'] = subcategories_by_category
  ctx['default_refresh'] = \
      skia_vars.GetGlobalVariable('default_webstatus_refresh')
  ctx['skia_repo'] = config_private.SKIA_GIT_URL

  active_master = config_private.Master.get_active_master()
  ctx['internal_port'] = active_master.master_port
  ctx['external_port'] = active_master.master_port_alt
  ctx['title_url'] = config_private.Master.Skia.project_url
  ctx['slave_hosts_cfg'] = slave_hosts_cfg.SLAVE_HOSTS
  ctx['slaves_cfg'] = slaves_cfg.SLAVES

  ctx['active_master_name'] = active_master.project_name
  ctx['master_revision'] = utils.get_current_revision()
  ctx['is_internal_view'] = request.host.port == ctx['internal_port']
  ctx['masters'] = []
  for master in config_private.Master.valid_masters:
    ctx['masters'].append({
      'name': master.project_name,
      'host': master.master_host,
      'internal_port': master.master_port,
      'external_port': master.master_port_alt,
    })
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

  def __init__(self, status, include_only_cq_trybots=False):
    JsonResource.__init__(self, status)
    for builder_name in self.status.getBuilderNames():
      if builder_name_schema.IsTrybot(builder_name) and (
          not include_only_cq_trybots or builder_name in slaves_cfg.CQ_TRYBOTS):
        self.putChild(builder_name,
                      BuilderJsonResource(status,
                                          status.getBuilder(builder_name)))


class CQRequiredStepsJsonResource(JsonResource):
  help = 'List the steps which cannot fail on the commit queue.'
  pageTitle = 'CQ Required Steps'

  def asDict(self, request):
    return {'cq_required_steps':
                skia_vars.GetGlobalVariable('cq_required_steps')}


def JsonStatusResourceInit(self, status):
  """ Override of buildbot.status.web.status_json.JsonStatusResource.__init__:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/third_party/buildbot_8_4p1/buildbot/status/web/status_json.py?view=markup

  We add trybots, cqtrybots, cq_required_steps (details below).
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
  ############################## Added by rmistry ##############################
  # Added to have a place to get the list of trybots run by the CQ.
  self.putChild('cqtrybots',
                TryBuildersJsonResource(status, include_only_cq_trybots=True))
  ##############################################################################
  ############################## Added by borenet ##############################
  # Added to have a place to get the list of steps which cannot fail on the CQ.
  self.putChild('cq_required_steps', CQRequiredStepsJsonResource(status))
  ##############################################################################

  ############################## Added by borenet ##############################
  # Added to have a way to determine which code revision the master is running.
  self.putChild('master_revision',
                master_revision.MasterCheckedOutRevisionJsonResource(status))
  running_rev = config_private.Master.get_active_master().running_revision
  self.putChild('master_running_revision',
                master_revision.MasterRunningRevisionJsonResource(
                    status=status, running_revision=running_rev))

  # This page gives the result of the most recent build for each builder.
  self.putChild('builder_statuses',
                builder_statuses.BuilderStatusesJsonResource(status))
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
      ########################## Added by rmistry ##########################
      if (job.get('requester') and not job['requester'].endswith('@google.com')
          and not job['requester'].endswith('@chromium.org')
          and not job['requester'] in TRYBOTS_REQUESTER_WHITELIST):
        # Reject the job only if the requester has an email not ending in
        # google.com or chromium.org
        raise BadJobfile(
            'TryJobRietveld rejecting job from %s' % job['requester'])
      ######################################################################
      ########################## Added by borenet ##########################
      if not (job.get('baseurl') and 
              config_private.Master.Skia.project_name.lower() in
                  job['baseurl']):
        raise BadJobfile('TryJobRietveld rejecting job with unknown baseurl: %s'
                         % job.get('baseurl'))
      ######################################################################
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
          # pylint: disable=W0212,W0612
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
          ########################## Added by rmistry #########################
          # Do not update Rietveld to mark the try job request as failed.
          # See https://code.google.com/p/chromium/issues/detail?id=224014 for
          # more context.
          # service.push('buildFinished', build=build)
          #####################################################################
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


def TryJobRietveldConstructor(
    self, name, pools, properties=None, last_good_urls=None,
    code_review_sites=None, project=None):
  try_job_base.TryJobBase.__init__(self, name, pools, properties,
                                   last_good_urls, code_review_sites)
  # pylint: disable=W0212
  endpoint = self._GetRietveldEndPointForProject(code_review_sites, project)
############################### Added by rmistry ###############################
  # rmistry: Increased the polling time from 10 seconds to 1 min because 10
  # seconds is too short for us. The RietveldPoller stops working if the time is
  # too short.
  # pylint: disable=W0212
  self._poller = try_job_rietveld._RietveldPoller(endpoint, interval=60)
################################################################################
  # pylint: disable=W0212
  self._valid_users = try_job_rietveld._ValidUserPoller(interval=12 * 60 * 60)
  self._project = project
  log.msg('TryJobRietveld created, get_pending_endpoint=%s '
          'project=%s' % (endpoint, project))

try_job_rietveld.TryJobRietveld.__init__ = TryJobRietveldConstructor


class SkiaGateKeeper(gatekeeper.GateKeeper):

  def isInterestingBuilder(self, builder_status):
    """ Override of gatekeeper.GateKeeper.isInterestingBuilder:
    http://src.chromium.org/viewvc/chrome/trunk/tools/build/scripts/master/gatekeeper.py?view=markup

    We modify it to actually check whether the builder should be considered by
    the GateKeeper, as indicated in its category name.
    """
    ret = (utils.GATEKEEPER_NAME in (builder_status.getCategory() or '') and
            chromium_notifier.ChromiumNotifier.isInterestingBuilder(self,
                builder_status))
    return ret

  def isInterestingStep(self, build_status, step_status, results):
    """ Override of gatekeeper.GateKeeper.isInterestingStep:
    http://src.chromium.org/viewvc/chrome/trunk/tools/build/scripts/master/gatekeeper.py?view=markup

    We modify it to comment out the SVN revision comparision to determine if the
    current build is older because Skia uses commit hashes.
    """
    # If we have not failed, or are not interested in this builder,
    # then we have nothing to do.
    if results[0] != FAILURE:
      return False

    # Check if the slave is still alive. We should not close the tree for
    # inactive slaves.
    slave_name = build_status.getSlavename()
    if slave_name in self.master_status.getSlaveNames():
      # @type self.master_status: L{buildbot.status.builder.Status}
      # @type self.parent: L{buildbot.master.BuildMaster}
      # @rtype getSlave(): L{buildbot.status.builder.SlaveStatus}
      slave_status = self.master_status.getSlave(slave_name)
      if slave_status and not slave_status.isConnected():
        log.msg('[gatekeeper] Slave %s was disconnected, '
                'not closing the tree' % slave_name)
        return False

    # If the previous build step failed with the same result, we don't care
    # about this step.
    previous_build_status = build_status.getPreviousBuild()
    if previous_build_status:
      step_name = self.getName(step_status)
      step_type = self.getGenericName(step_name)
      previous_steps = [step for step in previous_build_status.getSteps()
                        if self.getGenericName(self.getName(step)) == step_type]
      if len(previous_steps) == 1:
        if previous_steps[0].getResults()[0] == FAILURE:
          log.msg('[gatekeeper] Slave %s failed, but previously failed on '
                  'the same step (%s). So not closing tree.' % (
                      (step_name, slave_name)))
          return False
      else:
        log.msg('[gatekeeper] len(previous_steps) == %d which is weird' %
                len(previous_steps))

    # If check_revisions=False that means that the tree closure request is
    # coming from nightly scheduled bots, that need not necessarily have the
    # revision info.
    if not self.check_revisions:
      return True

    # If we don't have a version stamp nor a blame list, then this is most
    # likely a build started manually, and we don't want to close the
    # tree.
    latest_revision = build_utils.getLatestRevision(build_status)
    if not latest_revision or not build_status.getResponsibleUsers():
      log.msg('[gatekeeper] Slave %s failed, but no version stamp, '
              'so skipping.' % slave_name)
      return False

    # If the tree is open, we don't want to close it again for the same
    # revision, or an earlier one in case the build that just finished is a
    # slow one and we already fixed the problem and manually opened the tree.
    ############################### Added by rmistry ###########################
    # rmistry: Commenting out the below SVN revision comparision because Skia
    # uses commit hashes.
    # TODO(rmistry): Figure out how to ensure that previous builds do not close
    # the tree again.
    #
    # if latest_revision <= self._last_closure_revision:
    #   log.msg('[gatekeeper] Slave %s failed, but we already closed it '
    #           'for a previous revision (old=%s, new=%s)' % (
    #               slave_name, str(self._last_closure_revision),
    #               str(latest_revision)))
    #   return False
    ###########################################################################

    log.msg('[gatekeeper] Decided to close tree because of slave %s '
            'on revision %s' % (slave_name, str(latest_revision)))

    # Up to here, in theory we'd check if the tree is closed but this is too
    # slow to check here. Instead, take a look only when we want to close the
    # tree.
    return True


# Fix try_job_base.TryJobBase._EMAIL_VALIDATOR to handle *.info. This was fixed
# in https://codereview.chromium.org/216293005 but we need this monkeypatch to
# pick it up without a DEPS roll.
try_job_base.TryJobBase._EMAIL_VALIDATOR = re.compile(
    r'[a-zA-Z0-9][a-zA-Z0-9\.\+\-\_]*@[a-zA-Z0-9\.\-]+\.[a-zA-Z]{2,}$')
