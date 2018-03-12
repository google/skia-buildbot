#!/usr/bin/env python


import sys


syslog = '/var/log/syslog'

whitelist_lines = [
  ('metadata-server',  'Updated token: '),
  ('metadata-server',  'Token requested by '),
  ('get-oauth2-token', 'Wrote new auth token: '),
]


def match(line):
  for proc, pattern in whitelist_lines:
    if pattern in line:
      return line.split('jumphost', 1)[0] + proc + ': ' + pattern + line.split(pattern, 1)[1].rstrip()
  return False


def read_syslog():
  lines = []
  with open(syslog, 'rb') as f:
    for line in f:
     m = match(line)
     if m:
       lines.append(m)
  return lines


def filter_logs(ip, log_lines):
  tokens = []
  for line in log_lines:
    if ip and ip in line:
      tok = line.split(', serving ', 1)[1]
      tokens.append(tok)

  filtered = []
  for line in log_lines:
    if ip in line:
      filtered.append(line)
    else:
      for tok in tokens:
        if tok in line and not 'Token requested by' in line:
          filtered.append(line)
  return filtered


def main():
  lines = read_syslog()
  if len(sys.argv) > 1:
    lines = filter_logs(sys.argv[1], lines)
  for line in lines:
    print line


if __name__ == '__main__':
  main()
