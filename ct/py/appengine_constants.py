#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that contains constants for the skia-telemetry AppEngine WebApp."""


SKIA_TELEMETRY_WEBAPP = (
    'http://skia-tree-status.appspot.com/skia-telemetry/')

UPDATE_INFO_SUBPATH = 'update_telemetry_info'
GET_OLDEST_PENDING_TASK_SUBPATH = 'get_oldest_pending_task'

ADMIN_TASK_NAME = 'AdminTask'
CHROMIUM_BUILD_TASK_NAME = 'ChromiumBuildTask'
CHROMIUM_TRY_TASK_NAME = 'ChromiumTryTask'
TELEMETRY_TASK_NAME = 'TelemetryTask'
LUA_TASK_NAME = 'LuaTask'
SKIA_TRY_TASK_NAME = 'SkiaTryTask'

PAGESETS_ADMIN_TASK_NAME = 'Recreate Pagesets'
WEBPAGE_ARCHIVES_ADMIN_TASK_NAME = 'Recreate Webpage Archives'
