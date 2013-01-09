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


class SkiaTryMailNotifier(TryMailNotifier):
  """ The TryMailNotifier sends mail for every build by default. Since we use
  a single build master for both try builders and regular builders, this causes
  mail to be sent for every single build. So, we subclass TryMailNotifier here
  and add logic to prevent sending mail on anything but a try job. """

  def buildMessage(self, name, build, results):
    if build[0].source.patch:
      return TryMailNotifier.buildMessage(self, name, build, results)