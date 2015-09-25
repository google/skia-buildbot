#!/usr/bin/env python
# Copyright (c) 2015 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Python utility that triggers and waits for tasks to complete on CTFE."""

import base64
import hashlib
import json
import optparse
import requests
import sys
import time


CTFE_HOST = "http://ct.skia.org"
CTFE_QUEUE = CTFE_HOST + '/queue/'
CHROMIUM_PERF_TASK_POST_URI = CTFE_HOST + "/_/webhook_add_chromium_perf_task"
GET_CHROMIUM_PERF_RUN_STATUS_URI = CTFE_HOST + "/get_chromium_perf_run_status"
CHROMIUM_PERF_RUNS_HISTORY = CTFE_HOST + "/chromium_perf_runs/"
GCE_WEBHOOK_SALT_METADATA_URI = (
    "http://metadata/computeMetadata/v1/project/attributes/"
    "webhook_request_salt")


POLLING_FREQUENCY_SECS = 60  # 1 minute.
TRYBOT_DEADLINE_SECS = 24 * 60 * 60  # 24 hours.


class CtTrybotException(Exception):
  pass


def _CreateTaskJSON(options):
  """Creates a JSON representation of the requested task."""
  task_params = {}
  task_params["username"] = options.requester
  task_params["benchmark"] = options.benchmark
  task_params["platform"] = "Linux"
  task_params["page_sets"] = "10k"
  task_params["repeat_runs"] = "3"
  task_params["benchmark_args"] = "--output-format=csv-pivot-table"
  task_params["browser_args_nopatch"] = (
      "--disable-setuid-sandbox --enable-threaded-compositing "
      "--enable-impl-side-painting")
  task_params["browser_args_withpatch"] = (
      "--disable-setuid-sandbox --enable-threaded-compositing "
      "--enable-impl-side-painting")

  trybot_params = {}
  trybot_params["issue"] = options.issue
  trybot_params["patchset"] = options.patchset
  trybot_params["task"] = task_params
  return json.dumps(trybot_params)


def _GetWebhookSaltFromMetadata():
  """Gets webhook_request_salt from GCE's metadata server."""
  headers = {"Metadata-Flavor": "Google"}
  resp = requests.get(GCE_WEBHOOK_SALT_METADATA_URI, headers=headers)
  if resp.status_code != 200:
      raise CtTrybotException(
          'Return code from %s was %s' % (GCE_WEBHOOK_SALT_METADATA_URI,
                                          resp.status_code))
  return resp.text


def _TriggerTask(options):
  """Triggers the requested task on CTFE and returns the new task's ID."""
  task = _CreateTaskJSON(options)
  m = hashlib.sha512()
  m.update(task)
  m.update('notverysecret' if options.local else _GetWebhookSaltFromMetadata())
  encoded = base64.standard_b64encode(m.digest())

  headers = {
      "Content-type": "application/x-www-form-urlencoded",
      "Accept": "application/json",
      "X-Webhook-Auth-Hash": encoded}
  resp = requests.post(CHROMIUM_PERF_TASK_POST_URI, task, headers=headers)

  if resp.status_code != 200:
    raise CtTrybotException(
        'Return code from %s was %s' % (CHROMIUM_PERF_TASK_POST_URI,
                                        resp.status_code))
  try:
    ret = json.loads(resp.text)
  except ValueError, e:
    raise CtTrybotException(
        'Did not get a JSON response from %s: %s' % (
            CHROMIUM_PERF_TASK_POST_URI, e))
  return ret["taskID"]


def TriggerAndWait(options):
  task_id = _TriggerTask(options)

  print
  print 'Task %s has been successfull scheduled on CTFE (%s).' % (
      task_id, CHROMIUM_PERF_RUNS_HISTORY)
  print 'You will get an email once the task has been picked up by the server.'
  print
  print

  # Now poll CTFE till the task completes or till deadline is hit.
  time_started_polling = time.time()
  while True:
    if (time.time() - time_started_polling) > TRYBOT_DEADLINE_SECS:
      raise CtTrybotException(
          'Task did not complete in the deadline of %s seconds.' % (
              TRYBOT_DEADLINE_SECS))

    # Get the status of the task the trybot added.
    get_url = '%s?task_id=%s' % (GET_CHROMIUM_PERF_RUN_STATUS_URI, task_id)
    resp = requests.get(get_url)
    if resp.status_code != 200:
      raise CtTrybotException(
          'Return code from %s was %s' % (GET_CHROMIUM_PERF_RUN_STATUS_URI,
                                          resp.status_code))
    try:
      ret = json.loads(resp.text)
    except ValueError, e:
      raise CtTrybotException(
          'Did not get a JSON response from %s: %s' % (get_url, e))
    # Assert that the status is for the task we asked for.
    assert int(ret["taskID"]) == int(task_id)

    status = ret["status"]
    if status == "Completed":
      print
      print ('Your run was successfully completed. Please check your email for '
             'results of the run.')
      print
      return 0
    elif status == "Completed with failures":
      print
      raise CtTrybotException(
          'Your run was completed with failures. Please check your email for '
          'links to logs of the run.')

    print ('The current status of the task %s is "%s". You can view the size '
           'of the queue here: %s' % (task_id, status, CTFE_QUEUE))
    print 'Checking again after %s seconds' % POLLING_FREQUENCY_SECS
    print
    time.sleep(POLLING_FREQUENCY_SECS)


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--issue',
      help='The Rietveld CL number to get the patch from.')
  option_parser.add_option(
      '', '--patchset',
      help='The Rietveld CL patchset to use.')
  option_parser.add_option(
      '', '--requester',
      help='Email address of the user who requested this run.')
  option_parser.add_option(
      '', '--benchmark',
      help='The CT benchmark to run on the patch.')
  option_parser.add_option(
      '', '--local', default=False, action='store_true',
      help='Uses a dummy metadata salt if this flag is true else it tries to '
           'get the salt from GCE metadata.')
  options, unused_args = option_parser.parse_args()
  if (not options.issue or not options.patchset or not options.requester
      or not options.benchmark):
    option_parser.error('Must specify issue, patchset, requester and benchmark')

  sys.exit(TriggerAndWait(options))

