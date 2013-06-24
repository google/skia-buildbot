# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""AppEngine scripts to manage the skia-tree-status app.

   People with @chromium.org or @google.com logins can change the status that
   appears on the waterfall page. The previous statuses are kept in the
   AppEngine DB.
"""

from google.appengine.ext import webapp

import base_page
import commit_queue
import status
import sheriff
import skia_telemetry
import utils


class Warmup(webapp.RequestHandler):
  def get(self):
    """This handler is called as the initial request to 'warmup' the process."""
    pass


# Application configuration.
URLS = [
  ('/', status.MainPage),
  ('/allstatus/?', status.AllStatusPage),
  ('/banner-status/?', status.BannerStatusPage),
  ('/binary-status/?', status.BinaryStatusPage),
  ('/cq/receiver/?', commit_queue.Receiver),
  ('/cq/?', commit_queue.Summary),
  ('/cq/top', commit_queue.TopScore),
  ('/cq/([^/]+)/?', commit_queue.User),
  ('/cq/([^/]+)/(\d+)/?', commit_queue.Issue),
  ('/cq/([^/]+)/(\d+)/(\d+)/?', commit_queue.Issue),
  ('/current-sheriff/?', sheriff.CurrentSheriffPage),
  ('/sheriff/?', sheriff.SheriffPage),
  ('/skia-telemetry/?', skia_telemetry.LandingPage),
  ('/skia-telemetry/all_tasks?', skia_telemetry.AllTasks),
  ('/skia-telemetry/get_admin_tasks?', skia_telemetry.GetAdminTasksPage),
  ('/skia-telemetry/get_lua_tasks?', skia_telemetry.GetLuaTasksPage),
  ('/skia-telemetry/get_telemetry_tasks?',
   skia_telemetry.GetTelemetryTasksPage),
  ('/skia-telemetry/lua_script?', skia_telemetry.LuaScriptPage),
  ('/skia-telemetry/skia_telemetry_info_page?',
   skia_telemetry.TelemetryInfoPage),
  ('/skia-telemetry/update_admin_tasks?', skia_telemetry.UpdateAdminTasksPage),
  ('/skia-telemetry/update_telemetry_tasks?',
   skia_telemetry.UpdateTelemetryTasksPage),
  ('/skia-telemetry/update_lua_tasks?', skia_telemetry.UpdateLuaTasksPage),
  ('/skia-telemetry/update_telemetry_info?', skia_telemetry.UpdateInfoPage),
  ('/update_sheriffs_schedule', sheriff.update_sheriffs_schedule),
]
APPLICATION = webapp.WSGIApplication(URLS, debug=True)


# Do some one-time initializations.
base_page.bootstrap()
commit_queue.bootstrap()
status.bootstrap()
sheriff.bootstrap()
skia_telemetry.bootstrap()
utils.bootstrap()

