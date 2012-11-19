# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Skia-specific subclass of BuildStep """


from buildbot.status.logfile import STDOUT
from master.log_parser import retcode_command
import re
import utils


class SkiaBuildStep(retcode_command.ReturnCodeCommand):
  """ BuildStep wrapper for Skia. Allows us to define properties of BuildSteps
  to be used by ShouldDoStep. This is necessary because the properties referred
  to by BuildStep.getProperty() are scoped for the entire duration of the build.
  """
  def __init__(self, is_upload_step=False, is_rebaseline_step=False,
               get_props_from_stdout=None, **kwargs):
    """ Instantiates a new SkiaBuildStep.

    is_upload_step: boolean indicating whether this step should be skipped when
        the buildbot is not performing uploads.
    is_rebaseline_step: boolean indicating whether this step is required for
        rebaseline-only builds.
    get_props_from_stdout: optional dictionary. Keys are strings indicating
        build properties to set based on the output of this step. Values are
        strings containing regular expressions for parsing the property from
        the output of the step. 
    """
    self._is_upload_step = is_upload_step
    self._is_rebaseline_step = is_rebaseline_step
    self._get_props_from_stdout = get_props_from_stdout

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
    if self._get_props_from_stdout and cmd.rc == 0:
      log = cmd.logs['stdio']
      stdout = ''.join(log.getChunks([STDOUT], onlyText=True))
      self._changed_props = {}
      for property, regex in self._get_props_from_stdout.iteritems():
        matches = re.search(regex, stdout)
        if not matches:
          raise Exception('Unable to parse %s from stdout.' % property)
        groups = matches.groups()
        if len(groups) != 1:
          raise Exception('Multiple matches for "%s"' % regex)
        prop_value = groups[0]
        self.setProperty(property, prop_value, ''.join(self.description))
        self._changed_props[property] = prop_value
    retcode_command.ReturnCodeCommand.commandComplete(self, cmd)

  def getText(self, cmd, results):
    """ Override of BuildStep's getText method which appends any changed build
    properties to the description of the BuildStep. """
    text = self.description
    if self._changed_props:
      text.extend(['%s: %s' % (
          key, self._changed_props.get(key)) for key in self._changed_props])
    return text


def _HasProperty(step, property):
  """ Helper used by ShouldDoStep. Determine whether the given BuildStep has
  the requested property.

  step: an instance of BuildStep
  property: string, the property to test
  """
  try:
    step.getProperty(property)
    return True
  except:
    return False


def _CheckRebaselineChanges(changes, gm_image_subdir):
  """ Determine whether a set of changes consists of only files in 'gm-expected'
  and whether any of those files are in the given gm_image_subdir. Returns a
  tuple consisting of two booleans: whether or not the commit consists of only
  new baseline images, and whether or not baselines changed for the given
  platform.

  changes: a list of the changelists which are part of this build.
  gm_image_subdir: the subdirectory inside gm-expected which corresponds to this
      build slave's platform.
  """
  commit_is_only_baselines = True
  platform_changed = False
  for change in changes:
    for file in change.asDict()['files']:
      for subdir in utils.SKIA_PRIMARY_SUBDIRS:
        if subdir != 'gm-expected' and subdir in file['name']:
          commit_is_only_baselines = False
      if gm_image_subdir in file:
        platform_changed = True
  return commit_is_only_baselines, platform_changed


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