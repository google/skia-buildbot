# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""BuildFactory for a builder which recaptures the buildbot SKP repo."""


from skia_master_scripts import canary_factory

import builder_name_schema


class RecreateSKPsFactory(canary_factory.CanaryFactory):
  """Subclass of CanaryFactory which builds Chrome with LKGR Skia, builds Skia
  tools and then runs the webpages_playback.py scripts to recreate all buildbot
  SKPs."""

  def __init__(self, **kwargs):
    """Initialize the RecreateSKPsFactory."""
    canary_factory.CanaryFactory.__init__(self,
                                          path_to_skia=['third_party', 'skia'],
                                          flavor='chrome',
                                          build_subdir='src',
                                          build_targets=['chrome'],
                                          **kwargs)

  def Build(self, role=builder_name_schema.BUILDER_ROLE_HOUSEKEEPER,
            clobber=None):
    """Build and return the complete BuildFactory.

    role: string; type of builder
    clobber: boolean; indicating whether we should clean before building
    """
    if role != builder_name_schema.BUILDER_ROLE_HOUSEKEEPER:
      raise Exception('Canary builders must have role "%s"' %
                      builder_name_schema.BUILDER_ROLE_HOUSEKEEPER)

    # Build Chromium LKGR + Skia ToT.
    self.UpdateSteps()
    self.Compile(retry_without_werr_on_failure=True)

    # Invoke the do_skps_capture buildstep.
    skia_slave_scripts_path = self.TargetPath.join(
        '..', '..', '..', '..', '..', '..', 'slave',
        'skia_slave_scripts')
    self.AddSlaveScript(
        script=self.TargetPath.join(skia_slave_scripts_path,
                                    'do_skps_capture.py'),
        description='RecreateSKPs',
        args=['--page_sets', 'all',
              '--browser_executable', self.TargetPath.join(
                  'out', 'Debug', 'chrome')],
        timeout=None,
        halt_on_failure=True,
        workdir=self._workdir)

    self.Validate()  # Run the factory configuration test.
    return self
