# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Skia-specific subclass of BuildStep """


from buildbot.status.logfile import STDOUT
from master.log_parser import retcode_command
import re


class SkiaBuildStep(retcode_command.ReturnCodeCommand):
  """ BuildStep wrapper for Skia. Allows us to define properties of BuildSteps
  to be used by ShouldDoStep. This is necessary because the properties referred
  to by BuildStep.getProperty() are scoped for the entire duration of the build.
  """
  def __init__(self, is_upload_step=False, is_rebaseline_step=False,
               get_props_from_stdout=None, exception_on_failure=False,
               **kwargs):
    """ Instantiates a new SkiaBuildStep.

    is_upload_step: boolean indicating whether this step should be skipped when
        the buildbot is not performing uploads.
    is_rebaseline_step: boolean indicating whether this step is required for
        rebaseline-only builds.
    get_props_from_stdout: optional dictionary. Keys are strings indicating
        build properties to set based on the output of this step. Values are
        strings containing regular expressions for parsing the property from
        the output of the step.
    exception_on_failure: boolean indicating whether to raise an exception if
        this step fails. This causes the step to go purple instead of red, and
        causes the build to stop. Should be used if the build step's failure is
        typically transient or results from an infrastructure failure rather
        than a code change.
    """
    self._is_upload_step = is_upload_step
    self._is_rebaseline_step = is_rebaseline_step
    self._get_props_from_stdout = get_props_from_stdout
    self._exception_on_failure = exception_on_failure

    # self._changed_props will be a dictionary containing the build properties
    # which were updated by this BuildStep. Those properties will be displayed
    # in the label for this step.
    self._changed_props = None

    retcode_command.ReturnCodeCommand.__init__(self, **kwargs)
    self.name = ''.join(self.description)

  def IsUploadStep(self):
    return self._is_upload_step

  def IsRebaselineStep(self):
    return self._is_rebaseline_step

  def commandComplete(self, cmd):
    """ Override of BuildStep's commandComplete method which allows us to parse
    build properties from the output of this step. """
    if cmd.rc and self._exception_on_failure:
      raise Exception('Command marked exception_on_failure failed.')
    if self._get_props_from_stdout and cmd.rc == 0:
      log = cmd.logs['stdio']
      stdout = ''.join(log.getChunks([STDOUT], onlyText=True))
      self._changed_props = {}
      for prop, regex in self._get_props_from_stdout.iteritems():
        matches = re.search(regex, stdout)
        if not matches:
          raise Exception('Unable to parse %s from stdout.' % prop)
        groups = matches.groups()
        if len(groups) != 1:
          raise Exception('Multiple matches for "%s"' % regex)
        prop_value = groups[0]
        self.setProperty(prop, prop_value, ''.join(self.description))
        self._changed_props[prop] = prop_value
    retcode_command.ReturnCodeCommand.commandComplete(self, cmd)

  def getText(self, cmd, results):
    """ Override of BuildStep's getText method which appends any changed build
    properties to the description of the BuildStep. """
    text = self.description
    if self._changed_props:
      text.extend(['%s: %s' % (
          key, self._changed_props.get(key)) for key in self._changed_props])
    return text


def _HasProperty(step, prop):
  """ Helper used by ShouldDoStep. Determine whether the given BuildStep has
  the requested property.

  step: an instance of BuildStep
  prop: string, the property to test
  """
  try:
    step.getProperty(prop)
    return True
  # pylint: disable=W0702
  except:
    return False


def ShouldDoStep(step):
  """ At build time, use build properties to determine whether or not a step
  should be run or skipped.

  step: an instance of BuildStep which we may or may not run.
  """
  print step.build.getProperties()
  if not isinstance(step, SkiaBuildStep):
    return True

  # If this step uploads results (and thus overwrites the most recently uploaded
  # results), only run it on scheduled builds (i.e. most recent revision) or if
  # the "force_upload" property was set.
  if step.IsUploadStep() and \
      not _HasProperty(step, 'scheduler') and \
      not _HasProperty(step, 'force_upload'):
    return False

  # Unless we have determined otherwise, run the step.
  return True
