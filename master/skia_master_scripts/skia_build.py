# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Skia-specific subclass of Build. """

from master.factory.build import Build
from buildbot.status.results import SUCCESS, SKIPPED

import types


class SkiaBuild(Build):
  """ Skia-specific overrides for Build. """

  def setupBuild(self, expectations):
    Build.setupBuild(self, expectations)
    # Set the initial build result to 'SKIPPED'. If any step succeeds or fails,
    # this will be changed to reflect that. This setting causes builds in which
    # all steps were skipped *and* all previous runs of those steps were either
    # skipped or never run to be displayed as 'SKIPPED'.
    self.result = SKIPPED

  def stepDone(self, step_result, step):
    # This check is done in the superclass method in
    # buildbot.process.build.Build, but since we may modify step_result, we have
    # to do it here.
    if type(step_result) == types.TupleType:
      step_result, text = step_result
    else:
      text = None

    # If this step was skipped, set its result to the result of the last run.
    # Note that this only affects the success/failure status of the overall
    # build; the individual step will still be shown as SKIPPED, since we don't
    # modify step.result.
    try:
      if self.builder.builder_status.buildstepstatuses.has_key(step.name):
        previous_result = self.builder.builder_status.buildstepstatuses[step.name]
        if step_result == SKIPPED:
          step_result = previous_result
    except AttributeError:
      self.builder.builder_status.buildstepstatuses = {}
    if step_result != SKIPPED:
      self.builder.builder_status.buildstepstatuses[step.name] = step_result

    # Reset the result parameter to be used by the superclass method.
    if text:
      result_param = (step_result, text)
    else:
      result_param = step_result
    build_result = self.result

    # Superclass method which sets the result of the build based on the result
    # of each step.
    terminate = Build.stepDone(self, result_param, step)

    # Since we set the initial build status to SKIPPED instead of SUCCESS, and
    # since SKIPPED is "worse" than SUCCESS according to
    # buildbot.status.worst_status, it is possible that the superclass' stepDone
    # method did not properly set the build status as SUCCESS.
    if build_result == SKIPPED and step_result == SUCCESS:
      self.result = SUCCESS

    return terminate