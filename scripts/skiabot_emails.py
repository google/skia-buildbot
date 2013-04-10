# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Util to send emails.

To run this script you need a SMTP server installed locally.
This script is created to run from a local cronjob. The intended usage is that
the new sheriff should be emailed on Monday morning. The status email should be
sent out to the team also on Monday morning (this can also be alternatively done
at the end of the week on Friday).
"""

import datetime
import json
import os
import smtplib
import sys
import urllib2

# Set the PYTHONPATH for this script to include skia site_config.
sys.path.append(os.path.join(os.pardir, 'site_config'))
import skia_vars


SHERIFF_EMAIL_TYPE = 'sheriff'
STATUS_EMAIL_TYPE = 'status'
ALL_EMAIL_TYPES = (
    SHERIFF_EMAIL_TYPE,
    STATUS_EMAIL_TYPE
)

CURRENT_SHERIFF_JSON_URL = skia_vars.GetGlobalVariable('current_sheriff_url')
DEFAULT_EMAIL_SENDER = 'skia.buildbots@gmail.com'
ADDITIONAL_EMAIL_RECIPIENTS = ['skiabot@google.com']


def _GetSheriffDetails():
  """Returns the current sheriff and his/her schedule."""
  connection = urllib2.urlopen(CURRENT_SHERIFF_JSON_URL)
  sheriff_details = json.loads(connection.read())
  connection.close()
  return sheriff_details


def EmailSheriff():
  """Sends an email to the current sheriff."""
  sheriff_details = _GetSheriffDetails()
  if not sheriff_details:
    raise Exception('%s returned no data!' % CURRENT_SHERIFF_JSON_URL)

  sheriff_email = sheriff_details['username']
  sheriff_username = sheriff_email.split('@')[0]

  recipients = set(ADDITIONAL_EMAIL_RECIPIENTS)
  recipients.add(sheriff_email)

  body = """
Hi %s,


You are the Skia sheriff for the week (%s - %s).

Documentation for sheriffs is here:
https://sites.google.com/site/skiadocs/developer-documentation/tree-sheriff

The schedule for sheriffs is here:
http://skia-tree-status.appspot.com/sheriff

If you need to swap shifts with someone (because you are out sick or on vacation), please get approval from the person you want to swap with. Then send an email to skiabot@google.com to have someone make the change in the database (or directly ping rmistry).

Please let skiabot@google.com know if you have any other questions.


Thanks!
\n\n""" % (sheriff_username, sheriff_details['schedule_start'],
           sheriff_details['schedule_end'])

  subject = '%s is the new sheriff' % sheriff_username
  SendEmail(DEFAULT_EMAIL_SENDER, recipients, subject, body)


def EmailStatus():
  """Sends a status email to the Skia team."""

  print 'Implementation is not yet completed'
  exit(1)

  sheriff_details = _GetSheriffDetails()
  if not sheriff_details:
    raise Exception('%s returned no data!' % CURRENT_SHERIFF_JSON_URL)

  body = """
Hello all,


# Only show this if greater than 0.
Bugs currently breaking the build:
https://code.google.com/p/skia/issues/list?q=label:BreakingTheBuildbots

Total # of open Skia bugs:
Change from last week:
Bugs closed last week:

Clang static analysis results:
https://storage.cloud.google.com/chromium-skia-gm/static_analyzers/clang_static_analyzer/index.html

Skia Tree Sheriff for this week (%s - %s): %s

Total number of checkins last week:
All checkins are listed here:


Thanks!
\n\n""" % (sheriff_details['schedule_start'], sheriff_details['schedule_end'],
           sheriff_details['username'])

  recipients = set(ADDITIONAL_EMAIL_RECIPIENTS)
  # recipients.add('skia-team@google.com')
  subject = 'Weekly Skia status as of %s' % (
      datetime.datetime.now().strftime('%m/%d'))
  SendEmail(DEFAULT_EMAIL_SENDER, recipients, subject, body)


def SendEmail(sender, recipients, subject, body):
  """Sends an email using local SMTP server."""
  header = 'From: %s\nTo: %s\nSubject: %s\n\n' % (
      sender, ', '.join(recipients), subject)
  server = smtplib.SMTP('localhost')
  server.sendmail(sender, recipients, header + body)
  server.quit()


if __name__ == "__main__":
  if len(sys.argv) != 2 or sys.argv[1] == '-h' or sys.argv[1] == '--help':
    print 'Usage: python skiabot_emails.py email_type'
    print 'email_type can be one of: %s\n\n' % (ALL_EMAIL_TYPES,)
    sys.exit(1)

  email_type = sys.argv[1]
  if email_type not in ALL_EMAIL_TYPES:
    print '%s is not recognized. Please choose one of %s\n\n' % (
        email_type, ALL_EMAIL_TYPES)
    sys.exit(1)

  if email_type == SHERIFF_EMAIL_TYPE:
    EmailSheriff()
  elif email_type == STATUS_EMAIL_TYPE:
    EmailStatus()

