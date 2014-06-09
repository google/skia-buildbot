# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utility class to build the Skia master BuildFactory's for HouseKeeping bots.

Overrides SkiaFactory with Per-commit HouseKeeping steps."""


import builder_name_schema

from skia_master_scripts import factory as skia_factory


# TODO: The HouseKeepingPerCommitFactory uses shell scripts, so it would break
# on Windows. For now, we reply on the fact that the housekeeping bot always
# runs on a Linux machine.
class HouseKeepingPerCommitFactory(skia_factory.SkiaFactory):
  """Overrides for HouseKeeping per-commit builds."""
  def __init__(self, **kwargs):
    skia_factory.SkiaFactory.__init__(self, build_targets=['tools', 'gm', 'dm'],
                                      **kwargs)

  def Build(self, role=builder_name_schema.BUILDER_ROLE_HOUSEKEEPER,
            clobber=None):
    """Build and return the complete BuildFactory.

    role: string; type of builder.
    clobber: boolean indicating whether we should clean before building
    """
    if role != builder_name_schema.BUILDER_ROLE_HOUSEKEEPER:
      raise Exception('Housekeeping builders must have role "%s"' %
                      builder_name_schema.BUILDER_ROLE_HOUSEKEEPER)

    self.UpdateSteps()
    self.Compile(clobber)

    # TODO(borenet): Move these to a self-tests bot (http://skbug.com/2144)
    self.AddSlaveScript(script='run_tool_self_tests.py',
                        description='RunToolSelfTests')
    self.AddSlaveScript(script='run_gm_self_tests.py',
                        description='RunGmSelfTests')
    self.RunDM()

    # Run unittests for Anroid platform_tools
    self.AddSlaveScript(script='run_android_platform_self_tests.py',
                        description='RunAndroidPlatformSelfTests')

    # Check for static initializers.
    self.AddSlaveScript(script='detect_static_initializers.py',
                        description='DetectStaticInitializers')

    if not self._do_patch_step:  # Do not run Doxygen steps if try job.
      self.AddSlaveScript(script='generate_doxygen.py',
                          description='GenerateDoxygen')
      self.AddSlaveScript(script='upload_doxygen.py',
                          description='UploadDoxygen',
                          is_upload_render_step=True)

    self.AddSlaveScript(script='run_buildbot_self_tests.py',
                        description='BuildbotSelfTests')
    self.AddSlaveScript(script='check_compile_times.py',
                        description='CheckCompileTimes')
    self.Validate()
    return self
