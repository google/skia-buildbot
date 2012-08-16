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


class SkiaNotifier(MailNotifier):
  """This is Skia's status notifier."""

  def __init__(self, **kwargs):
    MailNotifier.__init__(self, **kwargs)

  def createEmail(self, msgdict, builderName, title, results, builds=None,
                  patches=None, logs=None):
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

