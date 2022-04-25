# Mock of browserbench Python driver script for testing purposes.
#
# See
# https://chromium.googlesource.com/chromium/src/+/refs/heads/main/tools/browserbench-webdriver/motionmark.py
#
# This mock takes the same arguments and emits valid Perf ingestion files with
# random values.
#
# Example:
#
#    python3 benchmark-mock.py  --output=$HOME/standin.json --browser=mock
from optparse import OptionParser

import json
import random
import sys

def ParseArgs():
  parser = OptionParser()
  parser.add_option('-b',
                    '--browser',
                    dest='browser',
                    help='The browser to use to run MotionMark in.')
  parser.add_option('-s',
                    '--suite',
                    dest='suite',
                    help='Run only the specified suite of tests.')
  parser.add_option('-e',
                    '--executable-path',
                    dest='executable',
                    help='Path to the executable to the driver binary.')
  parser.add_option('-a',
                    '--arguments',
                    dest='arguments',
                    help='Extra arguments to pass to the browser.')
  parser.add_option('-g',
                    '--githash',
                    dest='githash',
                    help='A git-hash associated with this run.')
  parser.add_option('-o',
                    '--output',
                    dest='output',
                    help='Path to the output json file.')
  (optargs, _) = parser.parse_args()
  optargs.suite = optargs.suite or 'Mock'
  optargs.githash = optargs.githash or 'deadbeef'
  return optargs

def ProduceOutput(data, output_file):
  print(json.dumps(data, sort_keys=True, indent=2, separators=(',', ': ')))
  if output_file:
    with open(output_file, 'w') as out:
      out.write(json.dumps(data))

def _extractScore():
  return [{
      'value': 'score',
      'measurement': random.random(),
  }]


def main():
  random.seed()
  optargs = ParseArgs()

  # Log sys.path to aid debugging launchctl.
  print('\n'.join(sys.path))

  results = {
      'version': 1,
      'git_hash': optargs.githash,
      "results": [
        {
          'key': {
              'test': 'mock',
              'browser': optargs.browser,
          },
          'measurements': {
              'score': _extractScore(),
          },
        }
      ]
  }

  ProduceOutput(results, optargs.output)

if __name__ == '__main__':
  main()
