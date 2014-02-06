# Copyright (c) 2010 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Bugdroid for Skia.

Upload Change List information to code.google.com Issue Tracker systems.
"""

import logging
import logging.handlers
import re

#import the GData libraries
import gdata.client
import gdata.data
import gdata.gauth
import gdata.projecthosting.client
import gdata.projecthosting.data

class Bugdroid(object):
  def __init__(self, email, password):
    """ Get email address and password to login to the Issue Tracker System. """
    self.email = email
    self.password = password
    self.bugdroid_client = None

  def _check_bug_id_synonyms(self, bug_ids):
    """ Check if a tracker synonym was used """
    chromium_os_syns = ['crosbug.com', 'chromeos', 'chrome-os', 'cros']
    chromium_syns = ['crbug.com', 'chrome', 'cr', 'crbug']
    chromeos_partner_syns = ['cros-partner', 'chromeos-partner',
                             'crosbug.com/p']
    updated_list = []
    for bug_id in bug_ids:
      bug_id_items = bug_id.split(':', 1)
      if bug_id_items[0] in chromium_os_syns:
        updated_list.append('chromium-os:%s' % bug_id_items[1])
      elif bug_id_items[0] in chromeos_partner_syns:
        updated_list.append('chrome-os-partner:%s' % bug_id_items[1])
      elif bug_id_items[0] in chromium_syns:
        updated_list.append('chromium:%s' % bug_id_items[1])
      else:
        updated_list.append(bug_id)
    return updated_list

  def _get_bug_id(self, content, default_tracker):
    """ Get bug ID from the text file. """
    entries = []
    for line in content.splitlines(False):
      match = re.match(r'^BUG *=(.*)', line)
      if match:
        for i in match.group(1).split(','):
          entries.extend(filter(None, [x.strip() for x in i.split()]))

    bug_ids = []
    last_tracker = default_tracker
    regex = (
      r'(http|https)://code.google.com/p/([^/]+)/issues/detail\?id=([0-9]+)'
      # The reason for the (\S+) below is because we accept things like
      # crosbug.com/p:123 in the synonym matcher
      r'|(\S+):([0-9]+)|(\b[0-9]+\b)')

    for new_item in entries:
      bug_numbers = re.findall(regex, new_item)
      for bug_tuple in bug_numbers:
        if bug_tuple[1] and bug_tuple[2]:
          bug_ids.append('%s:%s' % (bug_tuple[1], bug_tuple[2]))
          last_tracker = bug_tuple[1]
        elif bug_tuple[3] and bug_tuple[4]:
          bug_ids.append('%s:%s' % (bug_tuple[3], bug_tuple[4]))
          last_tracker = bug_tuple[3]
        elif bug_tuple[5]:
          bug_ids.append('%s:%s' % (last_tracker, bug_tuple[5]))
    bug_ids = self._check_bug_id_synonyms(bug_ids)
    bug_ids.sort(key=str.lower)
    return bug_ids

  def _get_author(self, content):
    """Get CL author from the CL """
    cl_author = None
    for line in content.splitlines(False):
      author_line = re.match(r'^Author:', line)
      if author_line:
        match = re.search('[\w.-]+@[\w.-]+', line)
        if match:
          cl_author =  match.group()
          return cl_author

  def _remove_leading_whitespace(self, content):
    return '\n'.join(line.strip() for line in content.splitlines() if line)

  def _compare_comments(self, comment1, comment2, compare_hash=False):
    if compare_hash:
      # Compare only the commit hashes
      logging.debug('Attempting to compare based on hashes alone.')
      try:
        comment1_hash = str.strip(comment1.splitlines()[0])
        # A lossy conversion is ok since unicode won't be in the commit line.
        comment2 = unicode(comment2).encode('ascii', 'replace')
        comment2_hash = str.strip(str(comment2).splitlines()[0])
      except Exception, e:
        logging.debug('Unable to convert commit line to strings. Error: %s' % e)
        # Play it safe otherwise we will update forever
        return True
      if comment1_hash == comment2_hash:
        return True
    else:
      comment1 = self._remove_leading_whitespace(unicode(comment1))
      comment2 = self._remove_leading_whitespace(unicode(comment2))
      if comment1 == comment2:
        return True

    return False

  def login(self):
    """Attempts to login to issue tracker
    Returns:
      True is login was successful; False otherwise.
    """
    login_success = False
    self.bugdroid_client = None
    try:
      self.bugdroid_client = gdata.projecthosting.client.ProjectHostingClient()
      self.bugdroid_client.client_login(self.email,
                                      self.password,
                                      source='google-skia-bugdroid-1.0',
                                      service='code')
      login_success = True
    except gdata.client.BadAuthentication, e:
      logging.debug('Unable to login to issue tracker.  Error: %s' % e)
    return login_success


  def _attempt_bug_update(self, project_name, issue_id, content):
    """Attempts to perform the update in issue tracker
    Args:
      project_name: name of the code.google.com project to update
      issue_id: the issue number to update
      comment: the comment to update
    Returns:
      0 if the bug is updated successfully; otherwise the status code.
    """

    # Bug status
    status = None
    status_words = ["Unconfirmed", "Untriaged", "Available", "Assigned",
                    "Started", "Upstream", "Fixed", "Verified", "Duplicate",
                    "WontFix", "FixUnreleased", "Invalid"]
    for line in content.splitlines(False):
      match = re.match(r'^STATUS *=(.*)', line, re.I)
      if match:
        status = match.group(1).title()
        logging.debug('Found "Status=%s" in the CL' % status)
        if status not in status_words:
          status = None
          logging.debug(
            ('No staus change. Please check status keyword.'
             '"Status="%s" was entered') % status)
        break

    author = self._get_author(content)

    try:
      self.bugdroid_client.update_issue(
          project_name,
          issue_id,
          self.email,
          comment=content,
          status=status,
          ccs=[author])
    except gdata.client.RequestError, e:
      logging.debug('Unable to update bug project %s issue %s.  Error: %s' %
                    (project_name, issue_id, e.message))
      return e.status
    logging.debug('Bug update for project %s issue %s was successful' %
                  (project_name, issue_id))
    return 0

  def update_bug(self, bug_id, content):
    """Adds a comment to the bug.
    Args:
      bug_id: Bug ID that is related to the current change list.
      issue_tracker_xml: Project hosting on Google Code uses xml entry to
      update bug
    Returns:
      0 if the bug is updated successfully, -1 if the bug was already updated;
      otherwise the server status code.
    """

    project_name, issue_id = bug_id.split(':', 1)
    try:
      query = gdata.projecthosting.client.Query(max_results='999')
      comments_feed = self.bugdroid_client.get_comments(project_name, issue_id,
                                                        query=query)
    except gdata.client.RequestError, e:
      logging.debug('Unable to locate project %s bug %s.  Error: %s' %
                    (project_name, issue_id, e.message))
      return e.status

    # If the bug only has an inital comment the comments_feed will be 0
    if len(comments_feed.entry) == 0:
      return self._attempt_bug_update(project_name, issue_id, content)

    # Convert the new comment to unicode
    check_hash_only = False
    try:
      unicode(content, errors='strict')
    except UnicodeDecodeError, e:
      logging.debug('Unable to convert the new comment to unicode.  '
                    'New comment:\n%s\nError: %s' % (content, e))
      check_hash_only = True

    for comment in comments_feed.entry:
      if self._compare_comments(content, comment.content.text,
                                compare_hash=check_hash_only):
        logging.debug('Bug for project %s issue %s has already been updated' %
                      (project_name, issue_id))
        return -1
    logging.debug('Going to attempt update for project %s issue %s' %
                  (project_name, issue_id))
    return self._attempt_bug_update(project_name, issue_id, content)

  def process_all_bugs(self, content, trackers, default_tracker):
    bug_ids = self._get_bug_id(content, default_tracker)
    # Check if bug ID is available. If not, no information would be updated
    # to the Issue Tracking system.
    for bug_id in bug_ids:
      logging.debug('Starting to process bug: %s' % bug_id)
      # Verify this is a supported tracker
      if bug_id.split(':', 1)[0] in trackers:
        # Sometimes the server returns an error like 500 and we need to retry.
        # If it is a 403 then we don't have access and there is no reason to
        # make a another attempt.
        for _ in xrange(3):
          status = self.update_bug(bug_id, content)
          if (status == 403) or (status == -1) or (status == 0):
            logging.debug('Exiting due to exit code: %d' % status)
            break
      else:
        logging.debug('Invalid tracker passed: %s' % bug_id)
    return len(bug_ids) != 0

