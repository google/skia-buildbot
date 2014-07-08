# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

import pickle
import socket
import struct

from buildbot.status.status_push import StatusPush
from buildbot.status import results


_DEFAULT_PORT = 2004    # Default port of pickled messages to Graphite
_SERVER_ADDRESS = 'skia-monitoring-b:%s' % _DEFAULT_PORT


def _sanitizeGraphiteNames(string):
  return string.replace('.', '_').replace(' ', '_')


class GraphiteStatusPush(StatusPush):
  """Uploads data to Graphite. Documentation for Graphite at
    http://graphite.readthedocs.org/en/latest/feeding-carbon.html """

  def __init__(self, serverAddr=_SERVER_ADDRESS):
    self.currentBuilds = {}
    self.serverAddress = serverAddr
    StatusPush.__init__(self, GraphiteStatusPush.pushGraphite)

  def pushGraphite(self):
    """Callback that StatusPush calls when something happens on the slaves."""
    events = self.queue.popChunk()
    valid_events = []

    def findInList(property_name, property_list):
      """Extracts a property from the status data. The data is arranged as
      triplets of information: internal name, data, external name."""
      for lst in property_list:
        if property_name == lst[0] or property_name == lst[2]:
          return lst[1]

    # Record only stepFinished events
    for event in events:
      if event['event'] == 'stepFinished':
        builder_name = findInList('buildername', event['payload']['properties'])
        master_name = findInList('master', event['payload']['properties'])
        key = '.'.join([
            'buildbot',
            _sanitizeGraphiteNames(master_name),
            _sanitizeGraphiteNames(builder_name),
            _sanitizeGraphiteNames(event['payload']['step']['name'])
        ])

        # Step duration.
        start = event['payload']['step']['times'][0]
        end = event['payload']['step']['times'][1]
        # The output is also a triplet, (name, (timestamp, value))
        valid_events.append(
            ('.'.join([key, 'duration']),
                (end, end - start)))

        # Step result.
        result = event['payload']['step'].get('results', [0])[0]
        failure = 0
        success = 0
        if result != results.SKIPPED:
          if result in (results.SUCCESS, results.WARNINGS):
            success = 1
          else:
            failure = 1
          valid_events.append(('.'.join((key, 'result')), (end, result)))
          valid_events.append(('.'.join((key, 'success')), (end, success)))
          valid_events.append(('.'.join((key, 'failure')), (end, failure)))

    if len(valid_events) <= 0:
      print 'GraphiteStatusPush: No valid events to send'
      return self.queueNextServerPush()
    else:
      print 'GraphiteStatusPush: %d events to send' % len(valid_events)

    # Send the events across a socket to the Graphite server
    try:
      sock = socket.socket()
      ip_address = self.serverAddress.split(':')[0]
      port_num = _DEFAULT_PORT
      if len(self.serverAddress.split(':')) > 1:
        port_num = int(self.serverAddress.split(':')[1])
      sock.connect((ip_address, port_num))
      for start in range(0, len(valid_events), 100):
        message = pickle.dumps(valid_events[start:start+100])
        header = struct.pack('!L', len(message))
        sock.sendall(header + message)
    except Exception:
      print 'GraphiteStatusPush: unable to connect to server'
      self.queue.insertBackChunk(events)

    return self.queueNextServerPush()


