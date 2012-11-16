# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Skia-specific subclass of Build. """

from master.factory.build import Build
from buildbot.status.results import SUCCESS, SKIPPED

import types


class SkiaBuild(Build):
  """ Skia-specific overrides for Build. """

  def stepDone(self, step_result, step):
    # Superclass method which sets the result of the build based on the result
    # of each step.
    terminate = Build.stepDone(self, step_result, step)

    # step_result is a tuple if the step has any extra information associated
    # with it.
    if type(step_result) == types.TupleType:
      step_result = step_result[0]

    # We want the build result to show SKIPPED if all steps which ran succeeded
    # but some steps were skipped. The superclass method just ignores the result
    # of skipped steps.
    if self.result == SUCCESS and step_result == SKIPPED:
      self.result = SKIPPED

    return terminate