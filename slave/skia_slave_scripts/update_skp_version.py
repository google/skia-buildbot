# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Create a CL to update the SKP version."""


import os
import re
import sys
import time

from build_step import BuildStep
from utils.git_utils import GIT
from utils import misc
from utils import shell_utils


PATH_TO_SKIA = os.path.join('third_party', 'skia')
SKIA_COMMITTER_EMAIL = 'borenet@google.com'
SKIA_COMMITTER_NAME = 'Eric Boren'
WAIT_TIME_BETWEEN_CHECKS = 300 # Seconds.

# Rather than calling the script using the shell, just import it.
sys.path.append(os.path.join(os.getcwd(), PATH_TO_SKIA))
from tools import gen_bench_expectations_from_codereview as bexpect


def wait():
  """Print a message and sleep for WAIT_TIME_BETWEEN_CHECKS seconds."""
  print 'Sleeping for %d seconds.' % WAIT_TIME_BETWEEN_CHECKS
  time.sleep(WAIT_TIME_BETWEEN_CHECKS)


class UpdateSkpVersion(BuildStep):
  def __init__(self, timeout=12800, **kwargs):
    super(UpdateSkpVersion, self).__init__(timeout=timeout, **kwargs)

  def _Run(self):
    with misc.ChDir(PATH_TO_SKIA):
      shell_utils.run([GIT, 'config', '--local', 'user.name',
                       SKIA_COMMITTER_NAME])
      shell_utils.run([GIT, 'config', '--local', 'user.email',
                       SKIA_COMMITTER_EMAIL])

      version_file = 'SKP_VERSION'
      skp_version = self._args.get('skp_version')
      with misc.GitBranch(branch_name='update_skp_version',
                          commit_msg='Update SKP version to %s' % skp_version,
                          commit_queue=not self._is_try) as branch:

        # First, upload a version of the CL with just the SKP version changed.
        with open(version_file, 'w') as f:
          f.write(skp_version)
        branch.commit_and_upload()

        # Trigger trybots.
        bots_to_trigger = []
        expectations_dir = os.path.join('expectations', 'bench')
        for expectations_file in os.listdir(expectations_dir):
          if os.path.isfile(os.path.join(expectations_dir, expectations_file)):
            m = re.match(r'bench_expectations_(?P<builder>.+)\.txt',
                         expectations_file)
            if m:
              bots_to_trigger.extend(['-b', m.group('builder') + '-Trybot'])

        try_cmd = [GIT, 'cl', 'try', '-m', 'tryserver.skia']
        try_cmd.extend(bots_to_trigger)
        shell_utils.run(try_cmd)

        # Find the issue number.
        output = shell_utils.run([GIT, 'cl', 'issue']).rstrip()
        codereview_url = re.match(r'Issue number: \d+ \((?P<url>.+)\)',
                                  output).group('url')

        # Wait for try results.
        print 'Waiting for trybot results...'
        wait()
        while not bexpect.all_trybots_finished(codereview_url):
          wait()

        # Add trybot results as new expectations.
        bexpect.gen_bench_expectations_from_codereview(codereview_url)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UpdateSkpVersion))
