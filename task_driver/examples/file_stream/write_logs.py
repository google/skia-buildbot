#!/usr/bin/env python

from __future__ import print_function
import sys
import time

with open(sys.argv[1], 'w') as f:
  print('Writing verbose logs to %s' % sys.argv[1])
  for i in range(10):
    f.write('%d\n' % i)
    f.flush()
    time.sleep(1)
