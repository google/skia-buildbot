#/usr/bin/python
# Copyright (c) 2010 The Chromium OS Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Tests for BugDroid"""

import datetime
import os
import time
import unittest

from bugdroid import Bugdroid

class TestBugDroid(unittest.TestCase):
  def get_password(self):
    parent = os.path.abspath(os.path.dirname(__file__))
    password_path = os.path.join(parent, '.bugdroid_password')
    return open(password_path, 'r').readline().strip()

  def test_single_bug(self):
    """ Test single issue BUG=1234"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=1234', 'foo')
    self.assertEqual('foo:1234', bugs[0])

  def test_multiple_default_tracker(self):
    """ Test multiple issues, default tracker BUG=1234,456,789"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=1234, 5678,890', 'foo')
    self.assertEqual(['foo:1234', 'foo:5678', 'foo:890'], bugs)

  def test_bugs_with_text(self):
    """ Test issues with text BUG=1234,789 fixes the issue"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=1234,789 fixes the issue', 'foo')
    self.assertEqual(['foo:1234', 'foo:789'], bugs)

  def test_bugs_with_multiple_trackers(self):
    """ Test multiple issues and trackers BUG=1234,goo:789, 891, goo:23"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=1234, goo:789, 891, goo:23', 'foo')
    self.assertEqual(['foo:1234', 'goo:23', 'goo:789', 'goo:891'], bugs)

  def test_bugs_with_multiple_lines(self):
    """ Test multiple issues and trackers BUG=1234\nBUG=234\nBUG=goo:789"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=1234\nBUG=234\nBUG=goo:789', 'foo')
    self.assertEqual(['foo:1234', 'foo:234', 'goo:789',], bugs)

  def test_bugs_with_embedded_text(self):
    """ Test multiple issues and trackers BUG=123 fixes login, 456"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=123 fixes login, 456', 'foo')
    self.assertEqual(['foo:123', 'foo:456',], bugs)

  def test_bugs_with_multiple_trackers_shorthand(self):
    """ Test multiple issues and trackers BUG=1234, 456,goo:78 89 90, 12"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=1234, 456,goo:78 89 90, 12', 'foo')
    self.assertEqual(['foo:1234', 'foo:456', 'goo:12', 'goo:78', 'goo:89',
                     'goo:90'], bugs)

  def test_bugs_with_multiple_trackers_space_delmited(self):
    """ Test multiple issues and trackers BUG=foo:12 foo:34 goo:45 goo:56"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=foo:12 foo:34 goo:45 goo:56', 'foo')
    self.assertEqual(['foo:12', 'foo:34', 'goo:45', 'goo:56'], bugs)

  def test_bugs_with_mix_match_and_text(self):
    """ Test mix and match with text BUG=12 fix bug goo:45, foo:14 goo:56, 9"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=12 fix bug goo:45, foo:14 goo:56, 9',
                                'foo')
    self.assertEqual(['foo:12', 'foo:14', 'goo:45', 'goo:56', 'goo:9'], bugs)

  def test_bugs_with_dashes_in_tracker_name(self):
    """ Test mix and match with text BUG=90, foo-bar:123, goo:45, 67 89"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=90, foo-bar:123, goo:45, 67 89', 'foo')
    self.assertEqual(['foo-bar:123', 'foo:90', 'goo:45', 'goo:67', 'goo:89'],
                     bugs)

  def test_bugs_with_URL(self):
    """ Test with URLs http://...id=13 foo:67 89 """
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id(
      'BUG=http://code.google.com/p/goo/issues/detail?id=13, foo:67 89', 'goo')
    self.assertEqual(['foo:67', 'foo:89', 'goo:13'], bugs)

  def test_bugs_with_http_URLs(self):
    """ Test with URLs http://..goo..id=13 67 89 foo:45 http://..poo..97"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id(
      'BUG=http://code.google.com/p/goo/issues/detail?id=13, 67 89 foo:45' +
      ' http://code.google.com/p/poo/issues/detail?id=97', 'foo')
    self.assertEqual(['foo:45', 'goo:13', 'goo:67', 'goo:89', 'poo:97'], bugs)

  def test_bugs_with_https_URLs(self):
    """ Test with URLs https://..goo..id=13 67 89 foo:45 https://..poo..97"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id(
      'BUG=https://code.google.com/p/goo/issues/detail?id=13, 67 89 foo:45' +
      ' https://code.google.com/p/poo/issues/detail?id=97', 'foo')
    self.assertEqual(['foo:45', 'goo:13', 'goo:67', 'goo:89', 'poo:97'], bugs)

  def test_bugs_with_http_and_https_URLs(self):
    """ Test with URLs https://..goo..id=13 67 89 foo:45 http://..poo..97"""
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id(
      'BUG=https://code.google.com/p/goo/issues/detail?id=13, 67 89 foo:45' +
      ' http://code.google.com/p/poo/issues/detail?id=97', 'foo')
    self.assertEqual(['foo:45', 'goo:13', 'goo:67', 'goo:89', 'poo:97'], bugs)

  def test_tracker_synonyms(self):
    """ Test the synonym matcher """
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=chromeos:123, crbug:123, ' +
                                  'crosbug.com/p:123', 'chromium-os')
    self.assertEqual(['chrome-os-partner:123', 'chromium-os:123',
                      'chromium:123'], bugs)

  def test_tracker_synonyms_mix(self):
    """ Test the synonym matcher with valid trackers and synonyms """
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    bugs = test_droid._get_bug_id('BUG=crosbug.com:123, cr:123, ' +
                                  'chrome-os-partner:123', 'chromium-os')
    self.assertEqual(['chrome-os-partner:123', 'chromium-os:123',
                      'chromium:123'], bugs)

  def test_git_svn_no_update_when_tracker_in_ignore_list(self):
    """ Test that git-svn updates for blacklisted trackers are not updated """
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password(),
                          svn_trackers_to_ignore=['chromium', 'nativeclient'])
    bugs = test_droid._get_bug_id('BUG=123\n' +
                                  'git-svn-id: svn://svn.chromium.org/'
                                  'chromium/trunk/src@107288 0039d316-1c4b-428',
                                  'chromium-os')
    self.assertEqual([], bugs)

  def test_git_svn_no_update_when_multiple_tracker_in_ignore_list(self):
    """ Test multiple git-svn updates for blacklisted trackers not updated """
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password(),
                          svn_trackers_to_ignore=['chromium', 'nativeclient'])
    bugs = test_droid._get_bug_id('BUG=123,chromium:456,chromium:789\n' +
                                  'git-svn-id: svn://svn.chromium.org/'
                                  'chromium/trunk/src@107288 0039d316-1c4b-428',
                                  'chromium-os')
    self.assertEqual([], bugs)

  def test_git_svn_update_explicit(self):
    """ Test that git-svn updates with the tracker set are updated """
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password(),
                          svn_trackers_to_ignore=['chromium', 'nativeclient'])
    bugs = test_droid._get_bug_id('BUG=123,chromium-os:234\n' +
                                  'git-svn-id: svn://svn.chromium.org/'
                                  'chromium/trunk/src@107288 0039d316-1c4b-428',
                                  'chromium-os')
    self.assertEqual(['chromium-os:234'], bugs)

  def test_git_svn_multiple_update_explicit(self):
    """ Test that git-svn multiple updates with the tracker set are updated """
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password(),
                          svn_trackers_to_ignore=['chromium', 'nativeclient'])
    bugs = test_droid._get_bug_id('BUG=123,chromium-os:234,456\n' +
                                  'git-svn-id: svn://svn.chromium.org/'
                                  'chromium/trunk/src@107288 0039d316-1c4b-428',
                                  'chromium-os')
    self.assertEqual(['chromium-os:234', 'chromium-os:456'], bugs)

  def test_git_svn_webrtc_no_update(self):
    """ Test the git-svn webrtc changes do not update bugs """
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password(),
                          svn_trackers_to_ignore=['chromium', 'webrtc'])
    bugs = test_droid._get_bug_id('BUG=123,456\n' +
                                  'git-svn-id: http://webrtc.googlecode.com/'
                                  'svn/trunk/src@1768 '
                                  '4adac7df-926f-26a2-2b94-8c16560cd09d',
                                  'chromium-os')
    self.assertEqual([], bugs)

  def test_not_adding_duplicate_entries(self):
    """ Test if the exact comment already exists we don't add it again """
    # Note: This test requires a .bugdroid_password file with the password
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    test_droid.login()
    timestamp = time.mktime(datetime.datetime.now().timetuple())
    new_comment = 'unittest updating bug with message %f' % timestamp
    result = test_droid.update_bug('chromium-os:8540', new_comment)
    self.assertEqual(0, result, 'The bug should have been updated')
    result = test_droid.update_bug('chromium-os:8540', new_comment)
    self.assertEqual(-1, result, 'The bug should not have been updated')

  def test_multiline_duplicate_entries(self):
    """ Test if comments different by newlines or whitespace are not added """
    # Note: This test requires a .bugdroid_password file with the password
    test_droid = Bugdroid('bugdroid1@chromium.org', self.get_password())
    test_droid.login()
    timestamp = time.mktime(datetime.datetime.now().timetuple())
    new_comment = 'unittest updating bug\n\n with message %f \n' % timestamp
    result = test_droid.update_bug('chromium-os:8540', new_comment)
    self.assertEqual(0, result, 'The bug should have been updated')
    diff_comment = 'unittest updating bug\n\n    with message %f \n' % timestamp
    result = test_droid.update_bug('chromium-os:8540', new_comment)
    self.assertEqual(-1, result, 'The bug should not have been updated')
    diff_comment = 'unittest updating bug\n\nwith message %f     \n' % timestamp
    result = test_droid.update_bug('chromium-os:8540', new_comment)
    self.assertEqual(-1, result, 'The bug should not have been updated')
    diff_comment = '\n\nunittest updating bug\nwith message %f \n\n' % timestamp
    result = test_droid.update_bug('chromium-os:8540', new_comment)
    self.assertEqual(-1, result, 'The bug should not have been updated')

if __name__ == '__main__':
    unittest.main()
