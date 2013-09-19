# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


from buildbot.process.properties import WithProperties
import factory
import os


class DRTCanaryFactory(factory.SkiaFactory):
  """ Subclass of Factory which builds Chrome with LKGR Skia, runs layout tests,
  then updates Skia to the most recent revision and runs the layout tests again,
  looking for diffs.
  """
  def __init__(self, path_to_skia, **kwargs):
    """ Instantiates a DRTCanaryFactory.

    path_to_skia: list of strings; indicates the path from the root of the
        project to the project's copy of Skia.
    """
    factory.SkiaFactory.__init__(self,
                                 flavor='chrome_canary',
                                 build_targets=['content_shell'],
                                 **kwargs)
    self._path_to_skia = self.TargetPath.join(*path_to_skia)

  # pylint: disable=W0221
  def Update(self, use_lkgr_skia=False):
    """ Override of Update() which may sync Skia to LKGR instead of
    self._revision, without setting got_revision. """
    self.AddSlaveScript(
        script=self.TargetPath.join(os.pardir, os.pardir, os.pardir, os.pardir,
                                    os.pardir, 'slave', 'skia_slave_scripts',
                                    '%s_update.py' % self._flavor),
        description='Update',
        timeout=None,
        halt_on_failure=True,
        is_upload_step=False,
        is_rebaseline_step=True,
        get_props_from_stdout=(
            {'chrome_revision': 'Chrome updated to (\w+)',
             'skia_base_rev': 'Skia updated to (\w+)'} if use_lkgr_skia
            else {'chrome_revision2': 'Chrome updated to (\w+)',
                  'got_revision': 'Skia updated to (\w+)'}),
        workdir='build',
        args=(['--use_lkgr_skia', 'True'] if use_lkgr_skia
              else ['--chrome_rev', WithProperties('%(chrome_revision)s')]))

  def PreTest(self):
    """ Step to run before running tests. """
    self.AddFlavoredSlaveScript(script='chrome_drt_canary_pretest.py',
                                description='PreTest')

  def RunWebkitTests(self, new_baseline=False):
    self.AddFlavoredSlaveScript(script='chrome_drt_canary_run_webkit_tests.py',
                                description='RunWebkitTests',
                                args=(['--new_baseline', 'True']
                                          if new_baseline
                                          else ['--write_results', 'True']))

  def UploadTestResults(self):
    self.AddFlavoredSlaveScript(script='chrome_drt_canary_upload_results.py',
                                description='UploadTestResults')

  def Build(self, **kwargs):
    self.UpdateScripts()
    self.Update(use_lkgr_skia=True)
    self.Compile(retry_without_werr_on_failure=True)
    self.PreTest()
    self.RunWebkitTests(new_baseline=True)
    self.Update(use_lkgr_skia=False)
    if self._do_patch_step:
      self.ApplyPatch()
    self.Compile(retry_without_werr_on_failure=True)
    self.RunWebkitTests(new_baseline=False)
    if self._do_upload_results:
      self.UploadTestResults()
    self.Validate()
    return self
