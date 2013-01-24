# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Monkeypatches to override upstream code. """

from master import try_job_base
from master import try_job_svn
from master.try_job_base import text_to_dict
from twisted.internet import defer
from twisted.python import log


################################################################################
############################# Trybot Monkeypatches #############################
################################################################################


@defer.deferredGenerator
def SubmitTryJobChanges(self, changes):
  """ Override of SVNPoller.submit_changes:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/scripts/master/try_job_svn.py?view=markup

  We modify it so that the patch file url is added to the build properties.
  This allows the slave to download the patch directly rather than receiving
  it from the master.
  """
  for chdict in changes:
    # pylint: disable=E1101
    parsed = self.parent.parse_options(text_to_dict(chdict['comments']))

    # 'fix' revision.
    # LKGR must be known before creating the change object.
    wfd = defer.waitForDeferred(self.parent.get_lkgr(parsed))
    yield wfd
    wfd.getResult()

    wfd = defer.waitForDeferred(self.master.addChange(
      author=','.join(parsed['email']),
      revision=parsed['revision'],
      comments='',
      properties={'patch_file_url': chdict['repository'] + '/' + \
                      chdict['files'][0]}))
    yield wfd
    change = wfd.getResult()

    self.parent.addChangeInner(chdict['files'], parsed, change.number)

try_job_svn.SVNPoller.submit_changes = SubmitTryJobChanges


def TryJobCreateBuildset(self, ssid, parsed_job):
  """ Override of TryJobBase.create_buildset:
  http://src.chromium.org/viewvc/chrome/trunk/tools/build/scripts/master/try_job_base.py?view=markup

  We modify it to verify that the requested builders are in the builder pool for
  this try scheduler. This prevents try requests from running on builders which
  are not registered as trybots. This apparently isn't a problem for Chromium
  since they use a separate try master.
  """
  log.msg('Creating try job(s) %s' % ssid)
  result = None
  for builder in parsed_job['bot']:
    if builder in self.pools[self.name]:
      result = self.addBuildsetForSourceStamp(ssid=ssid,
          reason=parsed_job['name'],
          external_idstring=parsed_job['name'],
          builderNames=[builder],
          properties=self.get_props(builder, parsed_job))
    else:
      log.msg('Rejecting try job for builder: %s' % builder)
  return result

try_job_base.TryJobBase.create_buildset = TryJobCreateBuildset