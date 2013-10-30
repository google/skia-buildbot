#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Module that contains constants for the skia-telemetry AppEngine WebApp."""


SKIA_TELEMETRY_WEBAPP = (
    'http://skia-tree-status.appspot.com/skia-telemetry/')

UPDATE_INFO_SUBPATH = 'update_telemetry_info'
GET_ADMIN_TASKS_SUBPATH = 'get_admin_tasks'
GET_CHROMIUM_BUILD_TASKS_SUBPATH = 'get_chromium_build_tasks'
GET_TELEMETRY_TASKS_SUBPATH = 'get_telemetry_tasks'
GET_LUA_TASKS_SUBPATH = 'get_lua_tasks'

CHROME_ADMIN_TASK_NAME = 'Rebuild Chrome'
PAGESETS_ADMIN_TASK_NAME = 'Recreate Pagesets'
PDFVIEWER_ADMIN_TASK_NAME = 'PDFViewer Diff'
WEBPAGE_ARCHIVES_ADMIN_TASK_NAME = 'Recreate Webpage Archives'
