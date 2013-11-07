from buildbot.status.web.base import HtmlResource
from buildbot.status.web.status_json import JsonResource
from twisted.internet import defer
from twisted.web.error import NoResource

import re


"""Status pages for Source Stamps."""


class SourceStampsStatusResource(HtmlResource):
  """Root for Source Stamp status pages. Has no content of its own."""

  def getChild(self, path, request):
    """Returns a SourceStampStatusResource for a given Source Stamp ID."""
    ssid_re = re.search(r'^/sourcestamp/?([0-9]+)', request.uri)
    if not ssid_re:
      NoResource('No such resource.')
    ssid = ssid_re.groups()[0]
    return SourceStampStatusResource(ssid)

  def content(self, request, cxt):
    """The Source Stamp status root has no content of its own."""
    return ''


class SourceStampsJsonResource(JsonResource):
  """Root for Source Stamp JSON statuses. Has no content of its own."""

  def getChild(self, path, request):
    """Returns a SourceStampJsonResource for a given Source Stamp ID."""
    ssid_re = re.search(r'^/json/sourcestamp/?([0-9]+)', request.uri)
    if not ssid_re:
      return NoResource('No such resource.')
    ssid = ssid_re.groups()[0]
    return SourceStampJsonResource(ssid, self.status)

  def asDict(self, request):
    """The Source Stamp status root has no content of its own."""
    return {}


class SourceStampJsonResource(JsonResource):
  """Provides information about the builds for a Source Stamp."""

  help = 'Get information about builds for a given SourceStamp.'
  pageTitle = 'Builds for SourceStamp'

  def __init__(self, ssid, status):
    """Initializes the SourceStampJsonResource."""
    JsonResource.__init__(self, status)
    self._ssid = ssid

  def asDict(self, request):
    """Returns a dictionary representing the builds for the Source Stamp. Takes
    the following format:

    {
      'buildsets': [
        {
          'bsid': 123,
          'build': {
            'bid': 123,
            'brid': 123,
            'finish_time': '2013-10-14 18:07:13.076779+00:00',
            'number': 22,
            'start_time': '2013-10-14 18:07:02.930166+00:00',
          },
          'builder': 'b_Update',
          'complete': True,
          'complete_at': '2013-10-14 18:07:13.299675+00:00',
          'external_idstring': None,
          'properties': {
            'dependencies': ([], 'Scheduler'),
            'scheduler': ('s_Update', 'Scheduler'),
          }
          'reason': 'The web-page "force build" button was pressed by "": ',
          'results': 0,
          'sourcestampid': 25,
          'submitted_at': '2013-10-14 18:07:02.519701+00:00',
        },

        ...

      ],
    }

    """

    def _got_builds(builds, buildsets, pending_buildsets):
      """Callback which runs after the Builds for each Build Request have been
      retrieved. Inserts the data for the builds (one per Buildset) into the
      buildsets dictionary and returns it."""
      assert len(builds) == len(buildsets)
      for i in xrange(len(buildsets)):
        try:
          if builds[i][1][0].get('start_time'):
            builds[i][1][0]['start_time'] = str(builds[i][1][0]['start_time'])
          if builds[i][1][0].get('finish_time'):
            builds[i][1][0]['finish_time'] = str(builds[i][1][0]['finish_time'])
          buildsets[i]['build'] = builds[i][1][0]
        except IndexError:
          # Occurs when the build hasn't started yet.
          buildsets[i]['build'] = None

      return {'buildsets': buildsets + pending_buildsets}

    def _got_buildreqs(buildreqs, buildsets, pending_buildsets):
      """Callback which runs after the Build Requests for each Buildset have
      been retrieved. Retrieves the Builds for each Build Request."""
      assert len(buildreqs) == len(buildsets)
      dl = []
      for i in xrange(len(buildreqs)):
        buildsets[i]['builder'] = buildreqs[i][1][0]['buildername']
        dl.append(
            master.db.builds.getBuildsForRequest(buildreqs[i][1][0]['brid']))
      d = defer.DeferredList(dl)
      d.addCallback(_got_builds, buildsets, pending_buildsets)
      return d

    def _got_props(props, buildsets, pending_buildsets):
      """Callback which runs after the Properties for a list of Buildsets have
      been retrieved. Retrieves the Build Requests for each Buildset."""
      assert len(props) == len(buildsets)
      dl = []
      for i in xrange(len(props)):
        buildsets[i]['properties'] = props[i][1]
        if buildsets[i].get('submitted_at'):
          buildsets[i]['submitted_at'] = str(buildsets[i]['submitted_at'])
        if buildsets[i].get('complete_at'):
          buildsets[i]['complete_at'] = str(buildsets[i]['complete_at'])
        dl.append(master.db.buildrequests.getBuildRequests(
            bsid=buildsets[i]['bsid']))
      d = defer.DeferredList(dl)
      d.addCallback(_got_buildreqs, buildsets, pending_buildsets)
      return d

    def _got_pending_buildsets(pending_buildsets, buildsets):
      """Callback which runs after the pending Buildsets for a given Source
      Stamp have been retrieved. Retrieves the properties for each Buildset."""
      dl = []
      for buildset in buildsets:
        dl.append(master.db.buildsets.getBuildsetProperties(buildset['bsid']))
      d = defer.DeferredList(dl)
      for pending_buildset in pending_buildsets:
        if pending_buildset.get('submitted_at'):
          pending_buildset['submitted_at'] = \
              str(pending_buildset['submitted_at'])
        if pending_buildset.get('complete_at'):
          pending_buildset['complete_at'] = str(pending_buildset['complete_at'])
        if not pending_buildset.get('properties'):
          pending_buildset['properties'] = {}
        pending_buildset['properties']['scheduler'] = \
            (pending_buildset['scheduler'], 'Scheduler')
        pending_buildset['properties']['dependencies'] = \
            (pending_buildset['dependencies'], 'Scheduler')
      d.addCallback(_got_props, buildsets, pending_buildsets)
      return d

    def _got_buildsets(buildsets):
      """Callback which runs after the Buildsets for a given Source Stamp have
      been retrieved. Retrieves the pending Buildsets for the Source Stamp."""
      if not buildsets:
        return {'error': 'No such Source Stamp'}
      d = master.skia_db.pending_buildsets.get_pending_buildsets(self._ssid)
      d.addCallback(_got_pending_buildsets, buildsets)
      return d

    master = request.site.buildbot_service.master
    d = master.db.buildsets.getBuildsetsForSourceStamp(self._ssid)
    d.addCallback(_got_buildsets)
    return d


class SourceStampStatusResource(HtmlResource):
  """Displays information about a SourceStamp."""

  def __init__(self, ssid, **kwargs):
    """Initialize the SourceStampStatusResource."""
    HtmlResource.__init__(self, **kwargs)
    self._ssid = ssid

  def content(self, request, cxt):
    """Serves the sourcestamp.html page, which loads its own data through the
    JSON interface."""
    cxt['ssid'] = self._ssid
    templates = request.site.buildbot_service.templates
    template = templates.get_template('sourcestamp.html')
    return template.render(cxt)

