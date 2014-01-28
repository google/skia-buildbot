#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Collect the results from the last run of the DEPS roll bot."""


from build_step import BuildStep, BuildStepWarning, BuildStepFailure
from utils import shell_utils

import config_private
import json
import os
import urllib2
import sys


CODEREVIEW_URL_TMPL = 'http://codereview.chromium.org/%s'


def get_last_finished_build(cached_builds, current_builds):
  """Find the last finished build, given the list of cached and running builds.

  Args:
      cached_builds: list of recent build numbers.
      current_builds: list of numbers of builds which are currently running.
  Returns:
      The greatest build number which is present in cached_builds but not
          current_builds.
  """
  for build_num in reversed(cached_builds):
    if build_num not in current_builds:
      return build_num
  return None


def get_codereview_issues(build_properties):
  """Find codereview issues for DEPS roll and control CLs in build properties.

  Args:
      build_properties: list of lists of the form:
          [ [prop_name, prop_value, prop_source], ... ]
  Returns:
      a two-tuple containing the URLs of codereview issues for the DEPS roll and
          its associated whitespace (control) change.
  """
  deps_roll_issue = None
  control_issue = None

  for name, value, _source in build_properties:
    if name == 'deps_roll_issue':
      deps_roll_issue = value
    if name == 'control_issue':
      control_issue = value

  return deps_roll_issue, control_issue


class CollectDEPSRollTrybotResults(BuildStep):
  def _Run(self):
    upstream_bot = self._args['upstream_bot']

    builder_json_url = 'http://%s:%s/json/builders/%s' % (
        config_private.Master.Skia.master_host,
        config_private.Master.Skia.master_port_alt,
        upstream_bot)

    print 'Obtaining builder data from %s' % builder_json_url
    builder_data = json.load(urllib2.urlopen(builder_json_url))

    latest_build_num = get_last_finished_build(builder_data['cachedBuilds'],
                                               builder_data['currentBuilds'])
    if not latest_build_num:
      raise BuildStepWarning('Could not find the last-finished build for %s!' %
                             upstream_bot)

    build_json_url = '%s/builds/%d' % (builder_json_url, latest_build_num)

    print 'Obtaining data for build #%d from %s' % (latest_build_num,
                                                    build_json_url)
    build_data = json.load(urllib2.urlopen(build_json_url))

    deps_roll_issue, whitespace_issue = \
        get_codereview_issues(build_data['properties'])

    if not (deps_roll_issue and whitespace_issue):
      raise BuildStepFailure('Could not find codereview issues!')

    print 'Found the following codereview issues:'
    print 'DEPS Roll: ', deps_roll_issue
    print 'Whitespace (control): ', whitespace_issue

    shell_utils.Bash(['python', os.path.join('tools', 'compare_codereview.py'),
                      CODEREVIEW_URL_TMPL % whitespace_issue,
                      CODEREVIEW_URL_TMPL % deps_roll_issue])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CollectDEPSRollTrybotResults))
