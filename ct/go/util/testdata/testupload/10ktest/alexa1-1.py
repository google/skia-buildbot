
# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
# pylint: disable=W0401,W0614

from telemetry.page import page as page_module
from telemetry.page import page_set as page_set_module


class TypicalAlexaPage(page_module.Page):

  def __init__(self, url, page_set):
    super(TypicalAlexaPage, self).__init__(url=url, page_set=page_set)
    self.user_agent_type = 'desktop'
    self.archive_data_file = '/b/storage/webpages_archive/10k/alexa1-1.json'

  def RunSmoothness(self, action_runner):
    action_runner.ScrollElement()

  def RunRepaint(self, action_runner):
    action_runner.RepaintContinuously(seconds=5)


class TypicalAlexaPageSet(page_set_module.PageSet):

  def __init__(self):
    super(TypicalAlexaPageSet, self).__init__(
      user_agent_type='desktop',
      archive_data_file='/b/storage/webpages_archive/10k/alexa1-1.json')

    urls_list = ['http://www.google.com']

    for url in urls_list:
      self.AddPage(TypicalAlexaPage(url, self))
