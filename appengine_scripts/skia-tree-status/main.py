# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""AppEngine scripts to manage the skia-tree-status app.

   People with @chromium.org or @google.com logins can change the status that
   appears on the waterfall page. The previous statuses are kept in the
   AppEngine DB.
"""

from google.appengine.ext import webapp

import status
import utils


class Warmup(webapp.RequestHandler):
  def get(self):
    """This handler is called as the initial request to 'warmup' the process."""
    pass


# Application configuration.
URLS = [
  ('/', status.MainPage),
  ('/all-status/?', status.AllStatusPage),
  ('/banner-status/?', status.BannerStatusPage),
  ('/binary-status/?', status.BinaryStatusPage),
]
APPLICATION = webapp.WSGIApplication(URLS, debug=True)


# Do some one-time initializations.
status.bootstrap()
utils.bootstrap()

