#!/usr/bin/env python
# Copyright (c) 2017 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Generate data to be used by the status-sk-demo."""


import datetime
import json
import urllib2


URL_TMPL = (
    'https://status-staging.skia.org/json/skia/incremental?from=%d&to=%d&n=35')


beginning_of_time = datetime.datetime.utcfromtimestamp(1505030640)
start = datetime.datetime.utcfromtimestamp(1506354185)
num_mins = 10
epoch = datetime.datetime.utcfromtimestamp(0)


def get(fro, to):
  """Retrieve a range of data from the server."""
  fro_ts = (fro - epoch).total_seconds() * 1000.0
  to_ts = (to - epoch).total_seconds() * 1000.0
  url = URL_TMPL % (fro_ts, to_ts)
  print url
  return json.load(urllib2.urlopen(url))


def get_all():
  """Retrieve all desired chunks of data from the server."""
  yield get(beginning_of_time, start)
  prevTs = start
  for _ in xrange(num_mins):
    nextTs = prevTs + datetime.timedelta(0, 60)
    yield get(prevTs, nextTs)
    prevTs = nextTs


def process(data):
  """Perform any desired processing of the JSON data."""
  # This is a no-op for now but is a placeholder in case we want to do things
  # like filter out certain task specs or rename things.
  return data


def finalize(idx, data):
  """Convert the data to a valid Javascript assignment."""
  return (('var data%d = ' % idx) +
          json.dumps(data, indent=4, sort_keys=True) + ';')


def main():
  """Download chunks of data, process them, and write to files."""
  for idx, data in enumerate(get_all()):
    with open('status-sk-demo-%d.json' % idx, 'w') as f:
      f.write(finalize(idx, process(data)))


if __name__ == '__main__':
  main()
