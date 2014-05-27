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
import builder_status
import commit_queue
import master_redirect
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
  ('/builder-status/?', builder_status.BuilderStatusPage),
  ('/builder-status/get_builder_statuses?',
   builder_status.GetBuilderStatusesPage),
  ('/buildbots/(.*)$', master_redirect.MasterBuildbotPage),
  ('/cq/receiver/?', commit_queue.Receiver),
  ('/cq/?', commit_queue.Summary),
  ('/cq/top', commit_queue.TopScore),
  ('/cq/([^/]+)/?', commit_queue.User),
  ('/cq/([^/]+)/(\d+)/?', commit_queue.Issue),
  ('/cq/([^/]+)/(\d+)/(\d+)/?', commit_queue.Issue),
  ('/current-sheriff/?', sheriff.CurrentSheriffPage),
  ('/lkgr?', status.LkgrPage),
  ('/next-sheriff/?', sheriff.NextSheriffPage),
  ('/query-sheriff/?', sheriff.QuerySheriffPage),
  ('/redirect/(.*)$', master_redirect.GenericRedirectionPage),
  ('/repo-serving/(.*)$', master_redirect.MasterRepoServingPage),
  ('/sheriff/?', sheriff.SheriffPage),
  ('/skia-telemetry/?', skia_telemetry.LandingPage),
  ('/skia-telemetry/admin_tasks?', skia_telemetry.AdminTasksPage),
  ('/skia-telemetry/all_tasks?', skia_telemetry.AllTasks),
  ('/skia-telemetry/chromium_builds?', skia_telemetry.ChromiumBuildsPage),
  ('/skia-telemetry/chromium_try?', skia_telemetry.ChromiumTryPage),
  ('/skia-telemetry/get_oldest_pending_task?',
   skia_telemetry.GetClusterTelemetryTasksPage),
  ('/skia-telemetry/lua_script?', skia_telemetry.LuaScriptPage),
  ('/skia-telemetry/pending_tasks?', skia_telemetry.PendingTasksPage),
  ('/skia-telemetry/skia_telemetry_info_page?',
   skia_telemetry.TelemetryInfoPage),
  ('/skia-telemetry/skia_try', skia_telemetry.SkiaTryPage),
  ('/skia-telemetry/update_admin_tasks?', skia_telemetry.UpdateAdminTasksPage),
  ('/skia-telemetry/update_chromium_build_tasks?',
   skia_telemetry.UpdateChromiumBuildTasksPage),
  ('/skia-telemetry/update_chromium_try_tasks?',
   skia_telemetry.UpdateChromiumTryTasksPage),
  ('/skia-telemetry/update_skia_try_tasks?',
   skia_telemetry.UpdateSkiaTryTasksPage),
  ('/skia-telemetry/update_telemetry_tasks?',
   skia_telemetry.UpdateTelemetryTasksPage),
  ('/skia-telemetry/update_lua_tasks?', skia_telemetry.UpdateLuaTasksPage),
  ('/skia-telemetry/update_telemetry_info?', skia_telemetry.UpdateInfoPage),
  ('/update_sheriffs_schedule', sheriff.update_sheriffs_schedule),
]
APPLICATION = webapp.WSGIApplication(URLS, debug=True)


# Do some one-time initializations.
base_page.bootstrap()
builder_status.bootstrap()
commit_queue.bootstrap()
status.bootstrap()
sheriff.bootstrap()
skia_telemetry.bootstrap()
utils.bootstrap()
