#!/usr/bin/env python

# Copyright 2018 Google LLC.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Search the syslog on a jumphost to determine when auth tokens changed."""


import sys


SYSLOG = '/var/log/syslog'

INCLUDE_LINES = [
  # (process-name,     pattern)
  ('metadata-server',  'Updated token: '),
  ('metadata-server',  'Token requested by '),
  ('get-oauth2-token', 'Wrote new auth token: '),
]


def transform_line(line):
  """Trim the log line and return it iff it matches INCLUDE_LINES."""
  for proc, pattern in INCLUDE_LINES:
    if pattern in line:
      # Log lines look like this:
      # pylint: disable=line-too-long
      # Mar 12 09:58:43 jumphost-win-02 metadata-server[5259]: I0312 09:58:43.756257    5259 server.go:87] Updated token: [redacted]
      timestamp = line.split('jumphost', 1)[0]
      suffix = line.split(pattern, 1)[1].rstrip()
      return timestamp + proc + ': ' + pattern + suffix
  return None


def read_syslog():
  """Read the syslog, returning any relevant lines."""
  lines = []
  with open(SYSLOG, 'rb') as f:
    for line in f:
     tf = transform_line(line)
     if tf:
       lines.append(tf)
  return lines


def filter_logs(ip, log_lines):
  """Filter the log lines to only those relevant to a particular IP address."""
  # First, collect all tokens used by the IP address.
  tokens = []
  for line in log_lines:
    if ip and ip in line:
      tok = line.split(', serving ', 1)[1]
      tokens.append(tok)

  # Filter to only lines which contain the IP address or one of its tokens.
  filtered = []
  for line in log_lines:
    if ip in line:
      filtered.append(line)
    else:
      for tok in tokens:
        # We don't care about other bots which used the token.
        if tok in line and not 'Token requested by' in line:
          filtered.append(line)
  return filtered


def main():
  """Read the syslog, filter to relevant lines, then print them."""
  lines = read_syslog()
  if len(sys.argv) > 1:
    lines = filter_logs(sys.argv[1], lines)
  for line in lines:
    print line


if __name__ == '__main__':
  main()
