# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Subclass of factory.SkiaFactory which builds some project other than Skia,
using the latest revision of Skia. """

import builder_name_schema
import factory
import os


class CanaryFactory(factory.SkiaFactory):
  """ BuildFactory for a project which uses Skia; updates the project to a
  known good revision, updates Skia to the latest revision, and verifies that
  the project builds.
  """
  def __init__(self, path_to_skia, flavor, **kwargs):
    """ Instantiates a CanaryFactory for a given project.

    path_to_skia: list of strings; indicates the path from the root of the
        project to the project's copy of Skia.
    """
    flavor = '%s_canary' % flavor
    factory.SkiaFactory.__init__(self, flavor=flavor, **kwargs)
    self._path_to_skia = self.TargetPath.join(*path_to_skia)

  def Update(self):
    self.AddSlaveScript(
        script=self.TargetPath.join('..', '..', '..', '..', '..', 'slave',
                                    'skia_slave_scripts',
                                    '%s_update.py' % self._flavor),
        description='Update',
        timeout=None,
        halt_on_failure=True,
        is_upload_step=False,
        is_rebaseline_step=True,
        get_props_from_stdout={'got_revision':
                                   'Skia updated to revision (\d+)',
                               'chrome_revision':
                                   'Chrome updated to (\w+)'},
        workdir='build')

  def ApplyPatch(self):
    # Note that, since Chrome only checks out the src, include, and gyp dirs,
    # any patch containing changes outside of those directories will fail to
    # apply.
    workdir = self.TargetPath.join(self._workdir, self._path_to_skia)
    path_to_script = [os.pardir for _ in workdir.split(self.TargetPath.sep)]
    path_to_script.extend([os.pardir, os.pardir, os.pardir, os.pardir,
                           'slave', 'skia_slave_scripts', 'apply_patch.py'])
    path_str = self.TargetPath.join(*path_to_script)
    factory.SkiaFactory.ApplyPatch(self,
                                   alternate_script=path_str,
                                   alternate_workdir=workdir)

  def RunGYP(self):
    self.AddSlaveScript(script='chrome_canary_run_gyp.py', description='RunGYP',
                        halt_on_failure=True,
                        args=['--gyp_defines',
                                  ' '.join('%s=%s' % (k, v) for k, v in
                                           self._gyp_defines.items())])

  def Build(self, **kwargs):
    # Spoof the role as a compile builder so that this factory only runs the
    # update and compile steps.
    return factory.SkiaFactory.Build(self,
        role=builder_name_schema.BUILDER_ROLE_BUILD, **kwargs)
