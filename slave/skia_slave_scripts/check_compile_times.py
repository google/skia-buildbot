#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Verify that the most recent compile builds are under an appropriate
threshold. """


from build_step import BuildStep, BuildStepFailure
from builder_name_schema import BUILDER_ROLE_BUILD
import config_private
import json
import sys
import urllib2


BUILD_MASTER_URL = 'http://%s:%s' % (
    config_private.Master.Skia.master_host,
    config_private.Master.Skia.master_port_alt)
COMPILE_TIME_LIMIT = 10000 # TODO(borenet): Make this something reasonable!


def IsBuildFinished(build_info):
  """ Determine whether the given build is finished, based on its start and end
  times. Returns True iff the start and end times are not None and are not zero.
  """
  return build_info['times'][0] and build_info['times'][1]


def GetBuildInfo(builder_name, build_num):
  """ Returns a dictionary containing information about a given build.

  builder_name: string; the name of the desired builder.
  build_num: string; the number of the desired build.
  """
  url = '%s/json/builders/%s/builds/%s' % (BUILD_MASTER_URL,
                                           builder_name,
                                           build_num)
  return json.load(urllib2.urlopen(url))


def GetLastFinishedBuildInfo(builder_name, cached_builds):
  """ Returns a dictionary containing information about the last finished build
  for the given builder.

  builder_name: string; the name of the desired builder.
  cached_builds: list of strings; candidate build numbers.
  """
  cached_builds.sort(reverse=True)
  for build_num in cached_builds:
    build_info = GetBuildInfo(builder_name, build_num)
    if IsBuildFinished(build_info):
      return build_info
  return None


class CheckCompileTimes(BuildStep):
  def _Run(self):
    # Obtain the list of Compile builders from the master.
    builders = json.load(urllib2.urlopen(BUILD_MASTER_URL + '/json/builders'))

    # Figure out which ones are too slow.
    too_slow = []
    no_builds = []
    longest_build = None
    for builder_name in builders.keys():
      if builder_name.startswith(BUILDER_ROLE_BUILD):
        cached_builds = sorted(builders[builder_name]['cachedBuilds'])
        if cached_builds:
          build_info = GetLastFinishedBuildInfo(builder_name, cached_builds)
          if build_info:
            duration = build_info['times'][1] - build_info['times'][0]
            summarized_build_info = {'builder': builder_name,
                                     'number': build_info['number'],
                                     'duration': duration}
            if duration > COMPILE_TIME_LIMIT:
              too_slow.append(summarized_build_info)
            if not longest_build or duration > longest_build['duration']:
              longest_build = summarized_build_info
          else:
            no_builds.append(builder_name)
    if longest_build:
      print 'Longest build: %(builder)s #%(number)s: %(duration)ds' % \
          longest_build
      print '%s/builders/%s/builds/%s' % (BUILD_MASTER_URL,
                                          longest_build['builder'],
                                          longest_build['number'])
      print
    if no_builds:
      print 'Warning: No builds found for the following builders:'
      for builder in no_builds:
        print '  %s' % builder
      print
    if too_slow:
      print 'The following builds exceeded the time limit of %ds:' % \
          COMPILE_TIME_LIMIT
      for summarized_build_info in too_slow:
        print '  %(builder)s #%(number)s: %(duration)ds' % summarized_build_info
      raise BuildStepFailure('Builds exceeded time limit.')



if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CheckCompileTimes))
