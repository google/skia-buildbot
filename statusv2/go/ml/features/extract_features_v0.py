#!/usr/bin/env python
#
# Copyright 2017 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Extract features from tasks."""


# WARNING: Do not change the output format of this file. You may copy this file
# and modify it but if you do so, be sure to change the VERSION constant.


import argparse
import datetime
import json
import os
import pyspark
import subprocess
import sys


# WARNING: If you copy/pasted this file, you MUST update the VERSION number.
VERSION = 0
GS_TMPL_RAW_LOG = 'gs://skia-swarming-logs/raw/%04d/%02d/%02d/%02d/%s.log'
GS_TMPL_RESULT = ('gs://skia-swarming-logs/features/with_2grams/v' +
                  str(VERSION) + '/%04d/%02d/%02d/%02d/%s.json')
PARTITIONS = 50


def get_raw_log_url(task):
  """Return the URL for the raw logs of this task."""
  dt = datetime.datetime.strptime(
      task['created'].split('.')[0], '%Y-%m-%dT%H:%M:%S')
  return GS_TMPL_RAW_LOG % (
      dt.year, dt.month, dt.day, dt.hour, task['swarmingTaskId'])


def get_result_url(task):
  """Return the URL where the results for this task should be uploaded."""
  dt = datetime.datetime.strptime(
      task['created'].split('.')[0], '%Y-%m-%dT%H:%M:%S')
  return GS_TMPL_RESULT % (
      dt.year, dt.month, dt.day, dt.hour, task['swarmingTaskId'])


def gsutil(*cmd):
  """Run the given gsutil command."""
  env = {'HOME': os.getcwd(),
         'PATH': os.environ.get('PATH')}
  return subprocess.check_output(
      ['gsutil'] + list(cmd), stderr=subprocess.STDOUT, env=env).rstrip()


def filter_tasks(tasks):
  """Remove any tasks for which we've already uploaded data."""
  rv = []
  for task in tasks:
    try:
      path = get_result_url(task)
      gsutil('ls', path)
    except subprocess.CalledProcessError as e:
      if 'One or more URLs matched no objects.' in e.output:
        rv.append(task)
      else:
        raise
  return rv


def tokenize(v):
  """Tokenize the given file."""
  # Just split on whitespace for now.
  return v.split()


def get_2grams(tokens):
  """Return a list of 2-tuples, the 2-grams in the list of tokens."""
  if len(tokens) < 2:
    return []
  rv = set()
  for idx, tok in enumerate(tokens[1:]):
    rv.add((tokens[idx], tok))
  return list(rv)


def get_logs_for_task(task):
  """Download the task log, extract 2-grams, and attach them to the task."""
  path = get_raw_log_url(task)
  log = gsutil('cat', path)
  tokens = tokenize(log)
  task['oneGrams'] = list(set(tokens))
  task['twoGrams'] = get_2grams(tokens)
  return task


def upload_task(task):
  """Upload the task to GS, return the GS URL."""
  filename = 'output.json'
  with open(filename, 'wb') as f:
    json.dump(task, f, indent=4)
  dest = get_result_url(task)
  try:
    gsutil('cp', '-Z', filename, dest)
  except subprocess.CalledProcessError as e:
    raise Exception(e.output)
  return dest


def process_task(task):
  """Download task log, merge it with the task data, upload the result."""
  return upload_task(get_logs_for_task(task))


def process_tasks(args, tasks):
  """Download task logs, merge them with the task data, and upload results.

  Returns:
    List of strings; the URLs of the uploaded files.
  """
  conf = pyspark.SparkConf()
  if args.profile:
    conf.set('spark.python.profile', 'true')
  sc = pyspark.SparkContext(conf=conf)

  # Farm out to the cluster.
  # Filter out tasks we already uploaded, key by ID.
  t = sc.parallelize(tasks, PARTITIONS)
  t = t.mapPartitions(filter_tasks, preservesPartitioning=True)
  t = t.map(lambda x: (x['id'], x))

  # Download logs, merge with task data, upload results.
  results = t.mapValues(process_task).collect()
  return [res[1] for res in results]


def main():
  """Download logs and merge them into each of the tasks in the JSON file."""
  # Parse args.
  parser = argparse.ArgumentParser()
  parser.add_argument('--tasks-json',
                      help='File containing tasks in JSON format.')
  parser.add_argument('--profile', default=False, action='store_true')
  parser.set_defaults(probs=False)
  args = parser.parse_args()

  with open(args.tasks_json) as f:
    tasks = json.load(f)

  results = process_tasks(args, tasks)
  print 'Uploaded %d of %d task files.' % (len(results), len(tasks))


if __name__ == '__main__':
  main()
