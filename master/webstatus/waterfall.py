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


from twisted.python import log
from twisted.internet import defer
import urllib

import time, locale

from buildbot import interfaces, util
from buildbot.status import builder, buildstep
from buildbot.changes import changes

from buildbot.status.web.base import Box, HtmlResource, IBox, ICurrentBox, \
     ITopBox, path_to_root, \
     map_branches
from buildbot.status.web.waterfall import earlier, \
                                          later, \
                                          insertGaps, \
                                          WaterfallHelp, \
                                          ChangeEventSource

class WaterfallStatusResource(HtmlResource):
  """This builds the main status page, with the waterfall display, and
  all child pages."""

  def __init__(self, categories=None, num_events=200, num_events_max=None):
    HtmlResource.__init__(self)
    self.categories = categories
    self.num_events = num_events
    self.num_events_max = num_events_max
    self.putChild("help", WaterfallHelp(categories))

  def getPageTitle(self, request):
    status = self.getStatus(request)
    p = status.getTitle()
    if p:
      return "BuildBot: %s" % p
    else:
      return "BuildBot"

  def getChangeManager(self, request):
    # TODO: this wants to go away, access it through IStatus
    return request.site.buildbot_service.getChangeSvc()

  def get_reload_time(self, request):
    if "reload" in request.args:
      try:
        reload_time = int(request.args["reload"][0])
        return max(reload_time, 15)
      except ValueError:
        pass
    return None

  def isSuccess(self, builderStatus):
    # Helper function to return True if the builder is not failing.
    # The function will return false if the current state is "offline",
    # the last build was not successful, or if a step from the current
    # build(s) failed.

    # Make sure the builder is online.
    if builderStatus.getState()[0] == 'offline':
      return False

    # Look at the last finished build to see if it was success or not.
    lastBuild = builderStatus.getLastFinishedBuild()
    if lastBuild and lastBuild.getResults() != builder.SUCCESS:
      return False

    # Check all the current builds to see if one step is already
    # failing.
    currentBuilds = builderStatus.getCurrentBuilds()
    if currentBuilds:
      for build in currentBuilds:
        for step in build.getSteps():
          if step.getResults()[0] == builder.FAILURE:
            return False

    # The last finished build was successful, and all the current builds
    # don't have any failed steps.
    return True

  def content(self, request, ctx):
    status = self.getStatus(request)
    master = request.site.buildbot_service.master

    # before calling content_with_db_data, make a bunch of database
    # queries.  This is a sick hack, but beats rewriting the entire
    # waterfall around asynchronous calls

    results = {}

    # recent changes
    changes_d = master.db.changes.getRecentChanges(40)
    def to_changes(chdicts):
      return defer.gatherResults([
          changes.Change.fromChdict(master, chdict) for chdict in chdicts ])
    changes_d.addCallback(to_changes)
    def keep_changes(changes):
      results['changes'] = changes
    changes_d.addCallback(keep_changes)

    # build request counts for each builder
    allBuilderNames = status.getBuilderNames(categories=self.categories)
    brstatus_ds = []
    brcounts = {}
    def keep_count(statuses, builderName):
      brcounts[builderName] = len(statuses)
    for builderName in allBuilderNames:
      builder_status = status.getBuilder(builderName)
      d = builder_status.getPendingBuildRequestStatuses()
      d.addCallback(keep_count, builderName)
      brstatus_ds.append(d)

    # wait for it all to finish
    d = defer.gatherResults([ changes_d ] + brstatus_ds)
    def call_content(_):
      return self.content_with_db_data(results['changes'],
          brcounts, request, ctx)
    d.addCallback(call_content)
    return d

  def content_with_db_data(self, changes, brcounts, request, ctx):
    status = self.getStatus(request)
    ctx['refresh'] = self.get_reload_time(request)

    # we start with all Builders available to this Waterfall: this is
    # limited by the config-file -time categories= argument, and defaults
    # to all defined Builders.
    allBuilderNames = status.getBuilderNames(categories=self.categories)
    builders = [status.getBuilder(name) for name in allBuilderNames]

    # but if the URL has one or more builder= arguments (or the old show=
    # argument, which is still accepted for backwards compatibility), we
    # use that set of builders instead. We still don't show anything
    # outside the config-file time set limited by categories=.
    showBuilders = request.args.get("show", [])
    showBuilders.extend(request.args.get("builder", []))
    if showBuilders:
      builders = [b for b in builders if b.name in showBuilders]

    # now, if the URL has one or category= arguments, use them as a
    # filter: only show those builders which belong to one of the given
    # categories.
    showCategories = request.args.get("category", [])
    if showCategories:
      builders = [b for b in builders if b.category in showCategories]

    # If the URL has the failures_only=true argument, we remove all the
    # builders that are not currently red or won't be turning red at the end
    # of their current run.
    failuresOnly = request.args.get("failures_only", ["false"])[0]
    if failuresOnly.lower() == "true":
      builders = [b for b in builders if not self.isSuccess(b)]

    (changeNames, builderNames, timestamps, eventGrid, sourceEvents) = \
        self.buildGrid(request, builders, changes)
        
    # start the table: top-header material
    locale_enc = locale.getdefaultlocale()[1]
    if locale_enc is not None:
      locale_tz = unicode(time.tzname[time.localtime()[-1]], locale_enc)
    else:
      locale_tz = unicode(time.tzname[time.localtime()[-1]])
    ctx['tz'] = locale_tz
    ctx['changes_url'] = request.childLink("../changes")

    bn = ctx['builders'] = []
            
    for name in builderNames:
      builder = status.getBuilder(name)
      top_box = ITopBox(builder).getBox(request)
      current_box = ICurrentBox(builder).getBox(status, brcounts)
      bn.append({'name': name,
                 'url': request.childLink("../builders/%s" %
                                          urllib.quote(name, safe='')),
                 'top': top_box.text,
                 'top_class': top_box.class_,
                 'status': current_box.text,
                 'status_class': current_box.class_,
                  })

    ctx.update(self.phase2(request, changeNames + builderNames, timestamps,
                           eventGrid, sourceEvents))

    def with_args(req, remove_args=[], new_args=[], new_path=None):
      # sigh, nevow makes this sort of manipulation easier
      newargs = req.args.copy()
      for argname in remove_args:
        newargs[argname] = []
      if "branch" in newargs:
        newargs["branch"] = [b for b in newargs["branch"] if b]
      for k, v in new_args:
        if k in newargs:
          newargs[k].append(v)
        else:
          newargs[k] = [v]
      newquery = "&amp;".join(["%s=%s" % (urllib.quote(k), urllib.quote(v))
                               for k in newargs
                               for v in newargs[k]
                               ])
      if new_path:
        new_url = new_path
      elif req.prepath:
        new_url = req.prepath[-1]
      else:
        new_url = ''
      if newquery:
        new_url += "?" + newquery
      return new_url

    if timestamps:
      bottom = timestamps[-1]
      ctx['nextpage'] = with_args(request, ["last_time"],
                                  [("last_time", str(int(bottom)))])


    helpurl = path_to_root(request) + "waterfall/help"
    ctx['help_url'] = with_args(request, new_path=helpurl)

    if self.get_reload_time(request) is not None:
      ctx['no_reload_page'] = with_args(request, remove_args=["reload"])

    template = request.site.buildbot_service.templates.get_template(
        "waterfall.html")
    data = template.render(**ctx)
    return data

  def buildGrid(self, request, builders, changes):
    debug = False
    # TODO: see if we can use a cached copy

    showEvents = False
    if request.args.get("show_events", ["false"])[0].lower() == "true":
      showEvents = True
    filterCategories = request.args.get('category', [])
    filterBranches = [b for b in request.args.get("branch", []) if b]
    filterBranches = map_branches(filterBranches)
    filterCommitters = [c for c in request.args.get("committer", []) if c]
    maxTime = int(request.args.get("last_time", [util.now()])[0])
    if "show_time" in request.args:
      minTime = maxTime - int(request.args["show_time"][0])
    elif "first_time" in request.args:
      minTime = int(request.args["first_time"][0])
    elif filterBranches or filterCommitters:
      minTime = util.now() - 24 * 60 * 60
    else:
      minTime = 0
    spanLength = 10  # ten-second chunks
    req_events = int(request.args.get("num_events", [self.num_events])[0])
    if self.num_events_max and req_events > self.num_events_max:
      maxPageLen = self.num_events_max
    else:
      maxPageLen = req_events

    # first step is to walk backwards in time, asking each column
    # (commit, all builders) if they have any events there. Build up the
    # array of events, and stop when we have a reasonable number.

    commit_source = ChangeEventSource(changes)

    lastEventTime = util.now()
    sources = [commit_source] + builders
    changeNames = ["changes"]
    builderNames = map(lambda builder: builder.getName(), builders)
    sourceNames = changeNames + builderNames
    sourceEvents = []
    sourceGenerators = []

    def get_event_from(g):
      try:
        while True:
          e = g.next()
          # e might be buildstep.BuildStepStatus,
          # builder.BuildStatus, builder.Event,
          # waterfall.Spacer(builder.Event), or changes.Change .
          # The showEvents=False flag means we should hide
          # builder.Event .
          if not showEvents and isinstance(e, builder.Event):
            continue

          if isinstance(e, buildstep.BuildStepStatus):
            # unfinished steps are always shown
            if e.isFinished() and e.isHidden():
              continue

          break
        event = interfaces.IStatusEvent(e)
        if debug:
          log.msg("gen %s gave1 %s" % (g, event.getText()))
      except StopIteration:
        event = None
      return event

    for s in sources:
      gen = insertGaps(s.eventGenerator(filterBranches,
                                        filterCategories,
                                        filterCommitters,
                                        minTime),
                       showEvents,
                       lastEventTime)
      sourceGenerators.append(gen)
      # get the first event
      sourceEvents.append(get_event_from(gen))
    eventGrid = []
    timestamps = []

    lastEventTime = 0
    for e in sourceEvents:
      if e and e.getTimes()[0] > lastEventTime:
        lastEventTime = e.getTimes()[0]
    if lastEventTime == 0:
      lastEventTime = util.now()

    spanStart = lastEventTime - spanLength
    debugGather = 0

    while 1:
      if debugGather:
        log.msg("checking (%s,]" % spanStart)
      # the tableau of potential events is in sourceEvents[]. The
      # window crawls backwards, and we examine one source at a time.
      # If the source's top-most event is in the window, is it pushed
      # onto the events[] array and the tableau is refilled. This
      # continues until the tableau event is not in the window (or is
      # missing).

      spanEvents = [] # for all sources, in this span. row of eventGrid
      firstTimestamp = None # timestamp of first event in the span
      lastTimestamp = None # last pre-span event, for next span

      for c in range(len(sourceGenerators)):
        events = [] # for this source, in this span. cell of eventGrid
        event = sourceEvents[c]
        while event and spanStart < event.getTimes()[0]:
          # to look at windows that don't end with the present,
          # condition the .append on event.time <= spanFinish
          if not IBox(event, None):
            log.msg("BAD EVENT", event, event.getText())
            assert 0
          if debug:
            log.msg("pushing", event.getText(), event)
          events.append(event)
          starts, finishes = event.getTimes()
          firstTimestamp = earlier(firstTimestamp, starts)
          event = get_event_from(sourceGenerators[c])
        if debug:
          log.msg("finished span")

        if event:
          # this is the last pre-span event for this source
          lastTimestamp = later(lastTimestamp, event.getTimes()[0])
        if debugGather:
          log.msg(" got %s from %s" % (events, sourceNames[c]))
        sourceEvents[c] = event # refill the tableau
        spanEvents.append(events)

      # only show events older than maxTime. This makes it possible to
      # visit a page that shows what it would be like to scroll off the
      # bottom of this one.
      if firstTimestamp is not None and firstTimestamp <= maxTime:
        eventGrid.append(spanEvents)
        timestamps.append(firstTimestamp)

      if lastTimestamp:
        spanStart = lastTimestamp - spanLength
      else:
        # no more events
        break
      if minTime is not None and lastTimestamp < minTime:
        break

      if len(timestamps) > maxPageLen:
        break

      # now loop

    # loop is finished. now we have eventGrid[] and timestamps[]
    if debugGather:
      log.msg("finished loop")
    assert(len(timestamps) == len(eventGrid))
    return (changeNames, builderNames, timestamps, eventGrid, sourceEvents)

  def phase2(self, request, sourceNames, timestamps, eventGrid, sourceEvents):

    if not timestamps:
      return dict(grid=[], gridlen=0)
    
    # first pass: figure out the height of the chunks, populate grid
    grid = []
    for i in range(1+len(sourceNames)):
      grid.append([])
    # grid is a list of columns, one for the timestamps, and one per
    # event source. Each column is exactly the same height. Each element
    # of the list is a single <td> box.
    lastDate = time.strftime("%d %b %Y", time.localtime(util.now()))
    for r in range(0, len(timestamps)):
      chunkstrip = eventGrid[r]
      # chunkstrip is a horizontal strip of event blocks. Each block
      # is a vertical list of events, all for the same source.
      assert(len(chunkstrip) == len(sourceNames))
      maxRows = reduce(lambda x, y: max(x, y),
                       map(lambda x: len(x), chunkstrip))
      for i in range(maxRows):
        if i != maxRows-1:
          grid[0].append(None)
        else:
          # timestamp goes at the bottom of the chunk
          stuff = []
          # add the date at the beginning (if it is not the same as
          # today's date), and each time it changes
          todayday = time.strftime("%a", time.localtime(timestamps[r]))
          today = time.strftime("%d %b %Y", time.localtime(timestamps[r]))
          if today != lastDate:
            stuff.append(todayday)
            stuff.append(today)
            lastDate = today
          stuff.append(time.strftime("%H:%M:%S", time.localtime(timestamps[r])))
          grid[0].append(Box(text=stuff, class_="Time", valign="bottom",
                             align="center"))

      # at this point the timestamp column has been populated with
      # maxRows boxes, most None but the last one has the time string
      for c in range(0, len(chunkstrip)):
        block = chunkstrip[c]
        assert(block != None) # should be [] instead
        for i in range(maxRows - len(block)):
          # fill top of chunk with blank space
          grid[c+1].append(None)
        for i in range(len(block)):
          # so the events are bottom-justified
          b = IBox(block[i]).getBox(request)
          b.parms['valign'] = "top"
          b.parms['align'] = "center"
          grid[c+1].append(b)
      # now all the other columns have maxRows new boxes too
    # populate the last row, if empty
    gridlen = len(grid[0])
    for i in range(len(grid)):
      strip = grid[i]
      assert(len(strip) == gridlen)
      if strip[-1] == None:
        if sourceEvents[i-1]:
          filler = IBox(sourceEvents[i-1]).getBox(request)
        else:
          # this can happen if you delete part of the build history
          filler = Box(text=["?"], align="center")
        strip[-1] = filler
      strip[-1].parms['rowspan'] = 1
    # second pass: bubble the events upwards to un-occupied locations
    # Every square of the grid that has a None in it needs to have
    # something else take its place.
    noBubble = request.args.get("nobubble", ['0'])
    noBubble = int(noBubble[0])
    if not noBubble:
      for col in range(len(grid)):
        strip = grid[col]
        if col == 1: # changes are handled differently
          for i in range(2, len(strip)+1):
            # only merge empty boxes. Don't bubble commit boxes.
            if strip[-i] == None:
              next = strip[-i+1]
              assert(next)
              if next:
                #if not next.event:
                if next.spacer:
                  # bubble the empty box up
                  strip[-i] = next
                  strip[-i].parms['rowspan'] += 1
                  strip[-i+1] = None
                else:
                  # we are above a commit box. Leave it
                  # be, and turn the current box into an
                  # empty one
                  strip[-i] = Box([], rowspan=1,
                                  comment="commit bubble")
                  strip[-i].spacer = True
              else:
                # we are above another empty box, which
                # somehow wasn't already converted.
                # Shouldn't happen
                pass
        else:
          for i in range(2, len(strip)+1):
            # strip[-i] will go from next-to-last back to first
            if strip[-i] == None:
              # bubble previous item up
              assert(strip[-i+1] != None)
              strip[-i] = strip[-i+1]
              strip[-i].parms['rowspan'] += 1
              strip[-i+1] = None
            else:
              strip[-i].parms['rowspan'] = 1

    # convert to dicts
    for i in range(gridlen):
      for strip in grid:
        if strip[i]:
          strip[i] = strip[i].td()

    return dict(grid=grid, gridlen=gridlen, no_bubble=noBubble, time=lastDate)

