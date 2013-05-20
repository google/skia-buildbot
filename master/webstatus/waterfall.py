# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Skia's override of buildbot.status.web.waterfall """


from buildbot import interfaces, util
from buildbot.status import builder as builder_status_module
from buildbot.status import buildstep
from buildbot.changes import changes as changes_module

from buildbot.status.web.base import Box, HtmlResource, IBox, ICurrentBox, \
     ITopBox, path_to_root, \
     map_branches
from buildbot.status.web.waterfall import earlier, \
                                          later, \
                                          insertGaps, \
                                          WaterfallHelp, \
                                          ChangeEventSource
from twisted.python import log
from twisted.internet import defer

import locale
import time
import urllib


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
    last_build = builderStatus.getLastFinishedBuild()
    if last_build and last_build.getResults() != builder_status_module.SUCCESS:
      return False

    # Check all the current builds to see if one step is already
    # failing.
    current_builds = builderStatus.getCurrentBuilds()
    if current_builds:
      for build in current_builds:
        for step in build.getSteps():
          if step.getResults()[0] == builder_status_module.FAILURE:
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
          changes_module.Change.fromChdict(master, chdict)
          for chdict in chdicts ])
    changes_d.addCallback(to_changes)
    def keep_changes(changes):
      results['changes'] = changes
    changes_d.addCallback(keep_changes)

    # build request counts for each builder
    all_builder_names = status.getBuilderNames(categories=self.categories)
    brstatus_ds = []
    brcounts = {}
    def keep_count(statuses, builder_name):
      brcounts[builder_name] = len(statuses)
    for builder_name in all_builder_names:
      builder_status = status.getBuilder(builder_name)
      d = builder_status.getPendingBuildRequestStatuses()
      d.addCallback(keep_count, builder_name)
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
    all_builder_names = status.getBuilderNames(categories=self.categories)
    builders = [status.getBuilder(name) for name in all_builder_names]

    # but if the URL has one or more builder= arguments (or the old show=
    # argument, which is still accepted for backwards compatibility), we
    # use that set of builders instead. We still don't show anything
    # outside the config-file time set limited by categories=.
    show_builders = request.args.get("show", [])
    show_builders.extend(request.args.get("builder", []))
    if show_builders:
      builders = [b for b in builders if b.name in show_builders]

    # now, if the URL has one or category= arguments, use them as a
    # filter: only show those builders which belong to one of the given
    # categories.
    show_categories = request.args.get("category", [])
    if show_categories:
      builders = [b for b in builders if b.category in show_categories]

    # If the URL has the failures_only=true argument, we remove all the
    # builders that are not currently red or won't be turning red at the end
    # of their current run.
    failures_only = request.args.get("failures_only", ["false"])[0]
    if failures_only.lower() == "true":
      builders = [b for b in builders if not self.isSuccess(b)]

    (change_names, builder_names, timestamps, event_grid, source_events) = \
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

    for name in builder_names:
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

    ctx.update(self.phase2(request, change_names + builder_names, timestamps,
                           event_grid, source_events))

    def with_args(req, remove_args=None, new_args=None, new_path=None):
      if not remove_args:
        remove_args = []
      if not new_args:
        new_args = []
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

    show_events = False
    if request.args.get("show_events", ["false"])[0].lower() == "true":
      show_events = True
    filter_categories = request.args.get('category', [])
    filter_branches = [b for b in request.args.get("branch", []) if b]
    filter_branches = map_branches(filter_branches)
    filter_committers = [c for c in request.args.get("committer", []) if c]
    max_time = int(request.args.get("last_time", [util.now()])[0])
    if "show_time" in request.args:
      min_time = max_time - int(request.args["show_time"][0])
    elif "first_time" in request.args:
      min_time = int(request.args["first_time"][0])
    elif filter_branches or filter_committers:
      min_time = util.now() - 24 * 60 * 60
    else:
      min_time = 0
    span_length = 10  # ten-second chunks
    req_events = int(request.args.get("num_events", [self.num_events])[0])
    if self.num_events_max and req_events > self.num_events_max:
      max_page_len = self.num_events_max
    else:
      max_page_len = req_events

    # first step is to walk backwards in time, asking each column
    # (commit, all builders) if they have any events there. Build up the
    # array of events, and stop when we have a reasonable number.

    commit_source = ChangeEventSource(changes)

    last_event_time = util.now()
    sources = [commit_source] + builders
    change_names = ["changes"]
    builder_names = map(lambda builder: builder.getName(), builders)
    source_names = change_names + builder_names
    source_events = []
    source_generators = []

    def get_event_from(g):
      try:
        while True:
          e = g.next()
          # e might be buildstep.BuildStepStatus,
          # builder.BuildStatus, builder.Event,
          # waterfall.Spacer(builder.Event), or changes.Change .
          # The show_events=False flag means we should hide
          # builder.Event .
          if not show_events and isinstance(e, builder_status_module.Event):
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
      gen = insertGaps(s.eventGenerator(filter_branches,
                                        filter_categories,
                                        filter_committers,
                                        min_time),
                       show_events,
                       last_event_time)
      source_generators.append(gen)
      # get the first event
      source_events.append(get_event_from(gen))
    event_grid = []
    timestamps = []

    last_event_time = 0
    for e in source_events:
      if e and e.getTimes()[0] > last_event_time:
        last_event_time = e.getTimes()[0]
    if last_event_time == 0:
      last_event_time = util.now()

    span_start = last_event_time - span_length
    debug_gather = 0

    while 1:
      if debug_gather:
        log.msg("checking (%s,]" % span_start)
      # the tableau of potential events is in source_events[]. The
      # window crawls backwards, and we examine one source at a time.
      # If the source's top-most event is in the window, is it pushed
      # onto the events[] array and the tableau is refilled. This
      # continues until the tableau event is not in the window (or is
      # missing).

      span_events = [] # for all sources, in this span. row of event_grid
      first_timestamp = None # timestamp of first event in the span
      last_timestamp = None # last pre-span event, for next span

      for c in range(len(source_generators)):
        events = [] # for this source, in this span. cell of event_grid
        event = source_events[c]
        while event and span_start < event.getTimes()[0]:
          # to look at windows that don't end with the present,
          # condition the .append on event.time <= spanFinish
          if not IBox(event, None):
            log.msg("BAD EVENT", event, event.getText())
            assert 0
          if debug:
            log.msg("pushing", event.getText(), event)
          events.append(event)
          starts, _ = event.getTimes()
          first_timestamp = earlier(first_timestamp, starts)
          event = get_event_from(source_generators[c])
        if debug:
          log.msg("finished span")

        if event:
          # this is the last pre-span event for this source
          last_timestamp = later(last_timestamp, event.getTimes()[0])
        if debug_gather:
          log.msg(" got %s from %s" % (events, source_names[c]))
        source_events[c] = event # refill the tableau
        span_events.append(events)

      # only show events older than max_time. This makes it possible to
      # visit a page that shows what it would be like to scroll off the
      # bottom of this one.
      if first_timestamp is not None and first_timestamp <= max_time:
        event_grid.append(span_events)
        timestamps.append(first_timestamp)

      if last_timestamp:
        span_start = last_timestamp - span_length
      else:
        # no more events
        break
      if min_time is not None and last_timestamp < min_time:
        break

      if len(timestamps) > max_page_len:
        break

      # now loop

    # loop is finished. now we have event_grid[] and timestamps[]
    if debug_gather:
      log.msg("finished loop")
    assert(len(timestamps) == len(event_grid))
    return (change_names, builder_names, timestamps, event_grid, source_events)

  def phase2(self, request, source_names, timestamps, event_grid,
             source_events):

    if not timestamps:
      return dict(grid=[], gridlen=0)

    # first pass: figure out the height of the chunks, populate grid
    grid = []
    for i in range(1+len(source_names)):
      grid.append([])
    # grid is a list of columns, one for the timestamps, and one per
    # event source. Each column is exactly the same height. Each element
    # of the list is a single <td> box.
    last_date = time.strftime("%d %b %Y", time.localtime(util.now()))
    for r in range(0, len(timestamps)):
      chunkstrip = event_grid[r]
      # chunkstrip is a horizontal strip of event blocks. Each block
      # is a vertical list of events, all for the same source.
      assert(len(chunkstrip) == len(source_names))
      max_rows = reduce(max, map(len, chunkstrip))
      for i in range(max_rows):
        if i != max_rows-1:
          grid[0].append(None)
        else:
          # timestamp goes at the bottom of the chunk
          stuff = []
          # add the date at the beginning (if it is not the same as
          # today's date), and each time it changes
          todayday = time.strftime("%a", time.localtime(timestamps[r]))
          today = time.strftime("%d %b %Y", time.localtime(timestamps[r]))
          if today != last_date:
            stuff.append(todayday)
            stuff.append(today)
            last_date = today
          stuff.append(time.strftime("%H:%M:%S", time.localtime(timestamps[r])))
          grid[0].append(Box(text=stuff, class_="Time", valign="bottom",
                             align="center"))

      # at this point the timestamp column has been populated with
      # max_rows boxes, most None but the last one has the time string
      for c in range(0, len(chunkstrip)):
        block = chunkstrip[c]
        assert(block != None) # should be [] instead
        for i in range(max_rows - len(block)):
          # fill top of chunk with blank space
          grid[c+1].append(None)
        for i in range(len(block)):
          # so the events are bottom-justified
          b = IBox(block[i]).getBox(request)
          b.parms['valign'] = "top"
          b.parms['align'] = "center"
          grid[c+1].append(b)
      # now all the other columns have max_rows new boxes too
    # populate the last row, if empty
    gridlen = len(grid[0])
    for i in range(len(grid)):
      strip = grid[i]
      assert(len(strip) == gridlen)
      if strip[-1] == None:
        if source_events[i-1]:
          filler = IBox(source_events[i-1]).getBox(request)
        else:
          # this can happen if you delete part of the build history
          filler = Box(text=["?"], align="center")
        strip[-1] = filler
      strip[-1].parms['rowspan'] = 1
    # second pass: bubble the events upwards to un-occupied locations
    # Every square of the grid that has a None in it needs to have
    # something else take its place.
    no_bubble = request.args.get("nobubble", ['0'])
    no_bubble = int(no_bubble[0])
    if not no_bubble:
      for col in range(len(grid)):
        strip = grid[col]
        if col == 1: # changes are handled differently
          for i in range(2, len(strip)+1):
            # only merge empty boxes. Don't bubble commit boxes.
            if strip[-i] == None:
              next_box = strip[-i+1]
              assert(next_box)
              if next_box:
                #if not next_box.event:
                if next_box.spacer:
                  # bubble the empty box up
                  strip[-i] = next_box
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

    return dict(grid=grid, gridlen=gridlen, no_bubble=no_bubble, time=last_date)

