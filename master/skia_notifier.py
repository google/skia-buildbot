# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""A StatusReceiver module to mail someone when a step warns/fails.

Since the behavior is very similar to the MailNotifier, we simply inherit from
it and also reuse some of its methods to send emails.
"""

# This module comes from $(TOPLEVEL_DIR)/third_party/buildbot_<VERSION> ,
# which must be in the PYTHONPATH.
from buildbot.status.mail import MailNotifier
from buildbot.status.results import Results
from master.try_mail_notifier import TryMailNotifier

import datetime
import re
import urllib


_COMMIT_QUEUE_AUTHOR_LINE = 'Author: '
_COMMIT_QUEUE_REVIEWERS_LINE = 'Reviewed By: '
_COMMIT_BOT = 'commit-bot@chromium.org'


class SkiaNotifier(MailNotifier):
  """This is Skia's status notifier."""

  def __init__(self, **kwargs):
    MailNotifier.__init__(self, **kwargs)

  def createEmail(self, msgdict, builderName, title, results, builds=None,
                  patches=None, logs=None):
    # Trybots have their own Notifier
    if 'Trybot' in builderName:
      return None

    m = MailNotifier.createEmail(self, msgdict, builderName, title,
        results, builds, patches, logs)

    if builds and builds[0].getSourceStamp().revision:
      m.replace_header('Subject',
          'buildbot %(result)s in %(title)s for r%(revision)s' % { 
              'result': Results[results],
              'title': title,
              'revision': builds[0].getSourceStamp().revision,
          })
    return m

  def buildMessage(self, name, builds, results):
    if self.sendToInterestedUsers and self.lookup:

      for build in builds:  # Loop through all builds we are emailing about
        blame_list = set(build.getResponsibleUsers())
        for change in build.getChanges():  # Loop through all changes in a build
          if change.comments and _COMMIT_BOT == change.who:
            # If the change has been submitted by the commit bot then find the
            # original author and the reviewers and add them to the blame list
            for commit_queue_line in (_COMMIT_QUEUE_AUTHOR_LINE,
                                      _COMMIT_QUEUE_REVIEWERS_LINE):
              users =  re.search(
                  '%s(.*?)\n' % commit_queue_line,
                  change.comments).group(1).split(',')
              blame_list = blame_list.union(users)
        # pylint: disable=C0301
        # Set the extended blamelist. It was originally set in
        # http://buildbot.net/buildbot/docs/0.8.4/reference/buildbot.process.build-pysrc.html
        # (line 339)
        build.setBlamelist(list(blame_list))

    return MailNotifier.buildMessage(self, name, builds, results)


def _ParseTimeStampFromURL(url):
  """ Parse a timestamp from a diff-file url.

  url: string; the url from which to parse the timestamp.
  """
  diff_file_name = urllib.unquote(url).split('/')[-1]
  m = re.search('\S+\.\S+\.(\d+)-(\d+)-(\d+)\s(\d+)\.(\d+)\.(\d+)\.\d+\.diff',
                diff_file_name)

  # If there are no matches or an incorrect number of matches, use the current
  # date as a default. We don't include the time because that would result in
  # many try result emails being sent with different subject lines. It is
  # preferable to group all emails for the same changelist on the same day (even
  # if they are from separate try requests).
  expected_num_matches = 6
  if not m or len(m.groups()) != expected_num_matches:
    now = datetime.datetime.now()
    return '%s-%s-%s' % (now.year, now.month, now.day)
  return '%s-%s-%s %s:%s:%s' % m.groups()


class SkiaTryMailNotifier(TryMailNotifier):
  """ The TryMailNotifier sends mail for every build by default. Since we use
  a single build master for both try builders and regular builders, this causes
  mail to be sent for every single build. So, we subclass TryMailNotifier here
  and add logic to prevent sending mail on anything but a try job. """

  def buildMessage(self, name, build, results):
    if build[0].source.patch:
      if not hasattr(build[0].source, 'timestamp'):
        build[0].source.timestamp = _ParseTimeStampFromURL(
            build[0].getProperty('patch_file_url'))
      return TryMailNotifier.buildMessage(self, name, build, results)
