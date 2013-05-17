# This file is part of Buildbot.  Buildbot is free software: you can
# redistribute it and/or modify it under the terms of the GNU General Public
# License as published by the Free Software Foundation, version 2.
#
# This program is distributed in the hope that it will be useful, but WITHOUT
# ANY WARRANTY; without even the implied warranty of MERCHANTABILITY or FITNESS
# FOR A PARTICULAR PURPOSE.  See the GNU General Public License for more
# details.
#
# You should have received a copy of the GNU General Public License along with
# this program; if not, write to the Free Software Foundation, Inc., 51
# Franklin Street, Fifth Floor, Boston, MA 02110-1301 USA.
#
# Copyright Buildbot Team Members

import time
import re
import urllib
from twisted.internet import defer
from buildbot import util
from buildbot.status import builder
from buildbot.status.web.base import HtmlResource
from buildbot.changes import changes
from buildbot.status.web.console import ANYBRANCH, \
                                        CacheStatus, \
                                        DevBuild, \
                                        DevRevision, \
                                        DoesNotPassFilter, \
                                        getInProgressResults, \
                                        getResultsClass, \
                                        TimeRevisionComparator, \
                                        IntegerRevisionComparator


class ConsoleStatusResource(HtmlResource):
  """Main console class. It displays a user-oriented status page.
  Every change is a line in the page, and it shows the result of the first
  build with this change for each slave."""

  def __init__(self, order_by_time=False):
    HtmlResource.__init__(self)

    self.status = None
    self.cache = CacheStatus()

    if order_by_time:
      self.comparator = TimeRevisionComparator()
    else:
      self.comparator = IntegerRevisionComparator()

  def getPageTitle(self, request):
    status = self.getStatus(request)
    title = status.getTitle()
    if title:
      return "BuildBot: %s" % title
    else:
      return "BuildBot"

  def getChangeManager(self, request):
    return request.site.buildbot_service.parent.change_svc

  ##
  ## Data gathering functions
  ##

  def getHeadBuild(self, builder):
    """Get the most recent build for the given builder.
    """
    build = builder.getBuild(-1)

    # HACK: Work around #601, the head build may be None if it is
    # locked.
    if build is None:
      build = builder.getBuild(-2)

    return build

  def fetchChangesFromHistory(self, status, max_depth, max_builds, debug_info):
    """Look at the history of the builders and try to fetch as many changes
    as possible. We need this when the main source does not contain enough
    sourcestamps.

    max_depth defines how many builds we will parse for a given builder.
    max_builds defines how many builds total we want to parse. This is to
        limit the amount of time we spend in this function.

    This function is sub-optimal, but the information returned by this
    function is cached, so this function won't be called more than once.
    """

    all_changes = list()
    build_count = 0
    for builder_name in status.getBuilderNames()[:]:
      if build_count > max_builds:
        break

      builder = status.getBuilder(builder_name)
      build = self.getHeadBuild(builder)
      depth = 0
      while build and depth < max_depth and build_count < max_builds:
        depth += 1
        build_count += 1
        sourcestamp = build.getSourceStamp()
        all_changes.extend(sourcestamp.changes[:])
        build = build.getPreviousBuild()

    debug_info["source_fetch_len"] = len(all_changes)
    return all_changes

  @defer.deferredGenerator
  def getAllChanges(self, request, status, debug_info):
    master = request.site.buildbot_service.master
    limit = min(100, max(1, int(request.args.get('limit', [25])[0])))
    wfd = defer.waitForDeferred(master.db.changes.getRecentChanges(limit))
    yield wfd
    chdicts = wfd.getResult()

    # convert those to Change instances
    wfd = defer.waitForDeferred(defer.gatherResults([
        changes.Change.fromChdict(master, chdict) for chdict in chdicts ]))
    yield wfd
    all_changes = wfd.getResult()

    all_changes.sort(key=self.comparator.getSortingKey())

    # Remove the dups
    prev_change = None
    new_changes = []
    for change in all_changes:
      rev = change.revision
      if not prev_change or rev != prev_change.revision:
        new_changes.append(change)
      prev_change = change
    all_changes = new_changes

    debug_info["source_len"] = len(all_changes)
    yield all_changes

  def getBuildDetails(self, request, builder_name, build):
    """Returns an HTML list of failures for a given build."""
    details = {}
    if not build.getLogs():
      return details

    for step in build.getSteps():
      (result, reason) = step.getResults()
      if result == builder.FAILURE:
        name = step.getName()

        # Remove html tags from the error text.
        strip_html = re.compile(r'<.*?>')
        stripped_details = strip_html.sub('', ' '.join(step.getText()))

        details['buildername'] = builder_name
        details['status'] = stripped_details
        details['reason'] = reason
        logs = details['logs'] = []

        if step.getLogs():
          for log in step.getLogs():
            logname = log.getName()
            logurl = request.childLink(
                "../builders/%s/builds/%s/steps/%s/logs/%s" %
                    (urllib.quote(builder_name),
                     build.getNumber(),
                     urllib.quote(name),
                     urllib.quote(logname)))
            logs.append(dict(url=logurl, name=logname))
    return details

  def getBuildsForRevision(self, request, builder, builder_name, last_revision,
                           num_builds, debug_info):
    """Return the list of all the builds for a given builder that we will
    need to be able to display the console page. We start by the most recent
    build, and we go down until we find a build that was built prior to the
    last change we are interested in."""

    revision = last_revision

    builds = []
    build = self.getHeadBuild(builder)
    number = 0
    while build and number < num_builds:
      debug_info["builds_scanned"] += 1
      number += 1

      # Get the last revision in this build.
      # We first try "got_revision", but if it does not work, then
      # we try "revision".
      got_rev = -1
      try:
        got_rev = build.getProperty("got_revision")
        if not self.comparator.isValidRevision(got_rev):
          got_rev = -1
      except KeyError:
        pass

      try:
        if got_rev == -1:
          got_rev = build.getProperty("revision")
        if not self.comparator.isValidRevision(got_rev):
          got_rev = -1
      except:
        pass

      # We ignore all builds that don't have last revisions.
      # TODO(nsylvain): If the build is over, maybe it was a problem
      # with the update source step. We need to find a way to tell the
      # user that his change might have broken the source update.
      if got_rev and got_rev != -1:
        details = self.getBuildDetails(request, builder_name, build)
        dev_build = DevBuild(got_rev, build, details,
                             getInProgressResults(build))
        builds.append(dev_build)

        # Now break if we have enough builds.
        current_revision = self.getChangeForBuild(build, revision)
        if self.comparator.isRevisionEarlier(
          dev_build, current_revision):
          break

      build = build.getPreviousBuild()

    return builds

  def getChangeForBuild(self, build, revision):
    if not build or not build.getChanges(): # Forced build
      return DevBuild(revision, build, None)
    
    for change in build.getChanges():
      if change.revision == revision:
        return change

    # No matching change, return the last change in build.
    changes = list(build.getChanges())
    changes.sort(key=self.comparator.getSortingKey())
    return changes[-1]

  def getAllBuildsForRevision(self, status, request, last_revision, num_builds,
                              categories, builders, debug_info):
    """Returns a dictionary of builds we need to inspect to be able to
    display the console page. The key is the builder name, and the value is
    an array of build we care about. We also returns a dictionary of
    builders we care about. The key is it's category.

    last_revision is the last revision we want to display in the page.
    categories is a list of categories to display. It is coming from the
        HTTP GET parameters.
    builders is a list of builders to display. It is coming from the HTTP
        GET parameters.
    """

    all_builds = dict()

    # List of all builders in the dictionary.
    builder_list = dict()

    debug_info["builds_scanned"] = 0
    # Get all the builders.
    builder_names = status.getBuilderNames()[:]
    for builder_name in builder_names:
      builder = status.getBuilder(builder_name)

      # Make sure we are interested in this builder.
      if categories and builder.category not in categories:
        continue
      if builders and builder_name not in builders:
        continue

      # We want to display this builder.
      category = builder.category or "default"
      # Strip the category to keep only the text before the first |.
      # This is a hack to support the chromium usecase where they have
      # multiple categories for each slave. We use only the first one.
      # TODO(nsylvain): Create another way to specify "display category"
      #     in master.cfg.
      category = category.split('|')[0]
      if not builder_list.get(category):
        builder_list[category] = []

      # Append this builder to the dictionary of builders.
      builder_list[category].append(builder_name)
      # Set the list of builds for this builder.
      all_builds[builder_name] = self.getBuildsForRevision(request,
                                                           builder,
                                                           builder_name,
                                                           last_revision,
                                                           num_builds,
                                                           debug_info)

    return (builder_list, all_builds)


  ##
  ## Display functions
  ##

  def displayCategories(self, builder_list, debug_info):
    """Display the top category line."""

    count = 0
    for category in builder_list:
      count += len(builder_list[category])

    categories = builder_list.keys()
    categories.sort()

    cs = []

    for category in categories:
      c = {}

      c["name"] = category

      # To be able to align the table correctly, we need to know
      # what percentage of space this category will be taking. This is
      # (#Builders in Category) / (#Builders Total) * 100.
      c["size"] = (len(builder_list[category]) * 100) / count
      cs.append(c)

    return cs

  def displaySlaveLine(self, status, builder_list, debug_info):
    """Display a line the shows the current status for all the builders we
    care about."""

    num_slaves = 0

    # Get the number of builders.
    for category in builder_list:
      num_slaves += len(builder_list[category])

    # Get the categories, and order them alphabetically.
    categories = builder_list.keys()
    categories.sort()

    slaves = {}

    # For each category, we display each builder.
    for category in categories:
      slaves[category] = []
      # For each builder in this category, we set the build info and we
      # display the box.
      for builder in builder_list[category]:
        s = {}
        s["color"] = "notstarted"
        s["pageTitle"] = builder
        s["url"] = "./builders/%s" % urllib.quote(builder, safe='() ')
        s["builderName"] = builder
        state, builds = status.getBuilder(builder).getState()
        # Check if it's offline, if so, the box is purple.
        if state == "offline":
          s["color"] = "offline"
        else:
          # If not offline, then display the result of the last
          # finished build.
          build = self.getHeadBuild(status.getBuilder(builder))
          while build and not build.isFinished():
            build = build.getPreviousBuild()

          if build:
            s["color"] = getResultsClass(build.getResults(), None, False)

        slaves[category].append(s)

    return slaves

  def displayStatusLine(self, builder_list, all_builds, revision, debug_info):
    """Display the boxes that represent the status of each builder in the
    first build "revision" was in. Returns an HTML list of errors that
    happened during these builds."""

    details = []
    num_slaves = 0
    for category in builder_list:
      num_slaves += len(builder_list[category])

    # Sort the categories.
    categories = builder_list.keys()
    categories.sort()

    builds = {}

    # Display the boxes by category group.
    for category in categories:

      builds[category] = []

      # Display the boxes for each builder in this category.
      for builder in builder_list[category]:
        introduced_in = None
        first_not_in = None

        cached_value = self.cache.get(builder, revision.revision)
        if cached_value:
          debug_info["from_cache"] += 1

          b = {}
          b["url"] = cached_value.url
          b["pageTitle"] = cached_value.pageTitle
          b["color"] = cached_value.color
          b["tag"] = cached_value.tag
          b["builderName"] = cached_value.builderName

          builds[category].append(b)

          if cached_value.details and cached_value.color == "failure":
            details.append(cached_value.details)

          continue

        # Find the first build that does not include the revision.
        for build in all_builds[builder]:
          if self.comparator.isRevisionEarlier(build, revision):
            first_not_in = build
            break
          else:
            introduced_in = build

        # Get the results of the first build with the revision, and the
        # first build that does not include the revision.
        results = None
        in_progress_results = None
        previous_results = None
        if introduced_in:
          results = introduced_in.results
          in_progress_results = introduced_in.inProgressResults
        if first_not_in:
          previous_results = first_not_in.results

        is_running = False
        if introduced_in and not introduced_in.isFinished:
          is_running = True

        url = "./waterfall"
        page_title = builder
        tag = ""
        current_details = {}
        if introduced_in:
          current_details = introduced_in.details or ""
          url = "./buildstatus?builder=%s&number=%s" % (urllib.quote(builder),
                                                        introduced_in.number)
          page_title += " "
          page_title += urllib.quote(' '.join(introduced_in.text), ' \n\\/:')

          builder_strip = builder.replace(' ', '')
          builder_strip = builder_strip.replace('(', '')
          builder_strip = builder_strip.replace(')', '')
          builder_strip = builder_strip.replace('.', '')
          tag = "Tag%s%s" % (builder_strip, introduced_in.number)

        if is_running:
          page_title += ' ETA: %ds' % (introduced_in.eta or 0)

        results_class = getResultsClass(results, previous_results, is_running,
                                        in_progress_results)

        b = {}
        b["url"] = url
        b["pageTitle"] = page_title
        b["color"] = results_class
        b["tag"] = tag
        b["builderName"] = builder

        builds[category].append(b)

        # If the box is red, we add the explaination in the details
        # section.
        if current_details and results_class == "failure":
          details.append(current_details)

        # Add this box to the cache if it's completed so we don't have
        # to compute it again.
        if results_class not in ("running", "running_failure",
                                 "notstarted"):
          debug_info["added_blocks"] += 1
          self.cache.insert(builder, revision.revision, results_class,
                            page_title, current_details, url, tag)

    return (builds, details)

  def filterRevisions(self, revisions, filter=None, max_revs=None):
    """Filter a set of revisions based on any number of filter criteria.
    If specified, filter should be a dict with keys corresponding to
    revision attributes, and values of 1+ strings"""
    if not filter:
      if max_revs is None:
        for rev in reversed(revisions):
          yield DevRevision(rev)
      else:
        for index, rev in enumerate(reversed(revisions)):
          if index >= max_revs:
            break
          yield DevRevision(rev)
    else:
      for index, rev in enumerate(reversed(revisions)):
        if max_revs and index >= max_revs:
          break
        try:
          for field, acceptable in filter.iteritems():
            if not hasattr(rev, field):
              raise DoesNotPassFilter
            if type(acceptable) in (str, unicode):
              if getattr(rev, field) != acceptable:
                raise DoesNotPassFilter
            elif type(acceptable) in (list, tuple, set):
              if getattr(rev, field) not in acceptable:
                raise DoesNotPassFilter
          yield DevRevision(rev)
        except DoesNotPassFilter:
          pass

  def displayPage(self, request, status, builder_list, all_builds, revisions,
                  categories, repository, branch, debug_info):
    """Display the console page."""
    # Build the main template directory with all the informations we have.
    subs = dict()
    subs["branch"] = branch or 'trunk'
    subs["repository"] = repository
    if categories:
      subs["categories"] = ' '.join(categories)
    subs["time"] = time.strftime("%a %d %b %Y %H:%M:%S",
                                 time.localtime(util.now()))
    subs["debugInfo"] = debug_info
    subs["ANYBRANCH"] = ANYBRANCH

    if builder_list:
      subs["categories"] = self.displayCategories(builder_list, debug_info)
      subs['slaves'] = self.displaySlaveLine(status, builder_list,
                                             debug_info)
    else:
      subs["categories"] = []

    subs['revisions'] = []

    # For each revision we show one line
    for revision in revisions:
      r = {}

      # Fill the dictionary with this new information
      r['id'] = revision.revision
      r['link'] = revision.revlink
      r['who'] = revision.who
      r['date'] = revision.date
      r['comments'] = revision.comments
      r['repository'] = revision.repository
      r['project'] = revision.project

      # Display the status for all builders.
      (builds, details) = self.displayStatusLine(builder_list,
                                                 all_builds,
                                                 revision,
                                                 debug_info)
      r['builds'] = builds
      r['details'] = details

      # Calculate the td span for the comment and the details.
      r["span"] = len(builder_list) + 2

      subs['revisions'].append(r)

    #
    # Display the footer of the page.
    #
    debug_info["load_time"] = time.time() - debug_info["load_time"]
    return subs


  def content(self, request, cxt):
    "This method builds the main console view display."

    reload_time = None
    # Check if there was an arg. Don't let people reload faster than
    # every 15 seconds. 0 means no reload.
    if "reload" in request.args:
      try:
        reload_time = int(request.args["reload"][0])
        if reload_time != 0:
          reload_time = max(reload_time, 15)
      except ValueError:
        pass

    request.setHeader('Cache-Control', 'no-cache')

    # Sets the default reload time to 60 seconds.
    if not reload_time:
      reload_time = 60

    # Append the tag to refresh the page.
    if reload_time is not None and reload_time != 0:
      cxt['refresh'] = reload_time

    # Debug information to display at the end of the page.
    debug_info = cxt['debuginfo'] = dict()
    debug_info["load_time"] = time.time()

    # get url parameters
    # Categories to show information for.
    categories = request.args.get("category", [])
    # List of all builders to show on the page.
    builders = request.args.get("builder", [])
    # Repo used to filter the changes shown.
    repository = request.args.get("repository", [None])[0]
    # Branch used to filter the changes shown.
    branch = request.args.get("branch", [ANYBRANCH])[0]
    # List of all the committers name to display on the page.
    dev_name = request.args.get("name", [])

    # and the data we want to render
    status = self.getStatus(request)

    # Keep only the revisions we care about.
    # By default we process the last 40 revisions.
    # If a dev name is passed, we look for the changes by this person in the
    # last 160 revisions.
    num_revs = int(request.args.get("revs", [40])[0])
    if dev_name:
      num_revs *= 4
    num_builds = num_revs

    # Get all changes we can find.  This is a DB operation, so it must use
    # a deferred.
    d = self.getAllChanges(request, status, debug_info)
    def got_changes(all_changes):
      debug_info["source_all"] = len(all_changes)

      rev_filter = {}
      if branch != ANYBRANCH:
        rev_filter['branch'] = branch
      if dev_name:
        rev_filter['who'] = dev_name
      if repository:
        rev_filter['repository'] = repository
      revisions = list(self.filterRevisions(all_changes, max_revs=num_revs,
                                            filter=rev_filter))
      debug_info["revision_final"] = len(revisions)

      # Fetch all the builds for all builders until we get the next build
      # after last_revision.
      builder_list = None
      all_builds = None
      if revisions:
        last_revision = revisions[len(revisions) - 1].revision
        debug_info["last_revision"] = last_revision

        (builder_list, all_builds) = self.getAllBuildsForRevision(status,
                                                                  request,
                                                                  last_revision,
                                                                  num_builds,
                                                                  categories,
                                                                  builders,
                                                                  debug_info)

      debug_info["added_blocks"] = 0
      debug_info["from_cache"] = 0

      if request.args.get("display_cache", None):
        data = ""
        data += "\nGlobal Cache\n"
        data += self.cache.display()
        return data

      cxt.update(self.displayPage(request, status, builder_list,
                                  all_builds, revisions, categories,
                                  repository, branch, debug_info))

      templates = request.site.buildbot_service.templates
      template = templates.get_template("console.html")
      data = template.render(cxt)

      # Clean up the cache.
      if debug_info["added_blocks"]:
        self.cache.trim()

      return data
    d.addCallback(got_changes)
    return d
