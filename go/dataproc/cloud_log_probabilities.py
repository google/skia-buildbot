#!/usr/bin/python


import argparse
import errno
import operator
import os
import pyspark
import shutil
import subprocess
import sys
import tempfile


# From what I can tell, these should be auto-detected when the job is submitted,
# but we seem to default to 2 partitions instead.
NUM_WORKERS = 50
NUM_CORES_PER_WORKER = 4
TOTAL_WORKER_CORES = NUM_WORKERS * NUM_CORES_PER_WORKER
UTILIZATION = 2.0  # Tune this for performance.
PARTITIONS = int(UTILIZATION * TOTAL_WORKER_CORES)

use_hdfs = False
use_wholeTextFiles = False


def build_urls(prefix, year, month, day, hour):
  f = lambda x: '%02d' % int(x)
  years = [y for y in (2016, 2017) if year == str(y) or year == '*']
  for y in years:
    months = [m for m in range(1, 13) if month == f(m) or month == '*']
    for m in months:
      days = [d for d in range(1, 32) if day == f(d) or day == '*']
      for d in days:
        hours = [h for h in range(24) if hour == f(h) or hour == '*']
        for h in hours:
          gs_dir = '%s/%04d/%02d/%02d/%02d' % (prefix, y, m, d, h)
          yield (gs_dir, gs_dir)  # Key/Value pairs.


def list_files(path):
  """List the files in the given dir."""
  if use_hdfs:
    try:
      out = subprocess.check_output(['hdfs', 'dfs', '-ls', path],
                                    stderr=subprocess.STDOUT).rstrip()
    except subprocess.CalledProcessError as e:
      if 'No such file or directory' in e.output:
        return []
      raise Exception('Command failed: %s\n%s' % (str(e), e.output))

    rv = []
    for line in out.splitlines():
      if line.endswith('.log'):
        rv.append(line.split()[-1])
    return rv
  else:
    try:
      env = {'HOME': os.getcwd(),
             'PATH': os.environ.get('PATH')}
      out = subprocess.check_output(['gsutil', 'ls', path],
                                    stderr=subprocess.STDOUT, env=env).rstrip()
    except subprocess.CalledProcessError as e:
      if 'One or more URLs matched no objects.' in e.output:
        return []
      raise Exception('Command failed: %s\n%s' % (str(e), e.output))
    rv = []
    for line in out.splitlines():
      if line.endswith('.log'):
        rv.append(line)
    return rv


def read_file(path):
  """Read the given file."""
  if use_hdfs:
    return subprocess.check_output(['hdfs', 'dfs', '-cat', path]).rstrip()
  else:
    env = {'HOME': os.getcwd(),
           'PATH': os.environ.get('PATH')}
    return subprocess.check_output(['gsutil', 'cat', path], env=env).rstrip()


def tokenize(v):
  """Tokenize the given file."""
  # Just split on whitespace for now.
  return v.split()


def get_2grams(tokens):
  """Return a list of 2-tuples, the 2-grams in the list of tokens."""
  if len(tokens) == 0:
    return []
  rv = []
  for idx, tok in enumerate(tokens[1:]):
    rv.append((tokens[idx-1], tok))
  return rv


def rekey(items):
  """Set the key of each item to the value for all items in the iterator."""
  for item in items:
    yield (item[1], item[1])


def swap(items):
  """Swap the key and value for all items in the iterator."""
  for item in items:
    yield (item[1], item[0])


def pair_value_with_one(items):
  """For each item, the new key is the old value and the new value is 1."""
  for item in items:
    yield (item[1], 1)


def process_logs(args):
  # Read and tokenize all files.
  gs_dest_full = args.dst

  conf = pyspark.SparkConf()
  
  if args.profile:
    conf.set('spark.python.profile', 'true')
  sc = pyspark.SparkContext(conf=conf)

  if use_wholeTextFiles:
    gs_source_full = '/'.join([
        args.src, args.year, args.month, args.day, args.hour])
    print 'Reading files from %s' % gs_source_full
    tokens_by_file = sc.wholeTextFiles(gs_source_full) \
        .map(lambda x: tokenize(x[1]))
  else:
    gs_dirs = sc.parallelize(build_urls(
        args.src, args.year, args.month, args.day, args.hour), PARTITIONS)
    file_list = gs_dirs.flatMapValues(list_files) \
        .mapPartitions(rekey)  # (gs_path, gs_path)
    bigrams_by_file = file_list \
        .flatMapValues(lambda x: get_2grams(tokenize(read_file(x))))  # (gs_path, bigram)

  # Find all 2-grams and their counts.
  bigrams = bigrams_by_file.mapPartitions(pair_value_with_one) # (bigram, 1)
  bigram_counts = bigrams.reduceByKey(lambda x, y: x + y)

  if args.probs:
    # Count the occurrences of each individual token.
    tokens = tokens_by_file.flatMap(lambda x: x) \
        .map(lambda x: (x, 1)) \
        .reduceByKey(lambda x, y: x + y)

    # For each pair of consecutive tokens, find the probability of the second
    # token occurring immediately after the first.

    # Key the 2-gram counts by the first token, ie. (tok1, (tok2, 2gramCount)).
    bigram_counts_keyed = bigram_counts.map(lambda x: (x[0][0], (x[0][1], x[1])))

    # This join gives us (tok1, (tok1Count, (tok2, 2gramCount))), after which we
    # can divide the 2gramCount by the tok1Count to obtain the probability, ie.
    # (tok1, (tok2, p(tok2))).
    probabilities = tokens.join(bigram_counts_keyed) \
        .map(lambda x: ((x[0], (x[1][1][0], float(x[1][1][1]) / float(x[1][0])))))

    # Report results.
    for tok1, nextPrediction in probabilities.take(10):
      print '%s\t->\t%s\t(p = %2f)' % (tok1, nextPrediction[0], nextPrediction[1])

    probabilities.saveAsTextFile(gs_dest_full)
  else:
    # Just report the 2-gram counts.
    # Sort descending.
    bigram_counts_reversed = bigram_counts.mapPartitions(swap)
    bigram_counts_sort = bigram_counts_reversed.sortByKey(ascending=False)

    # Print the 10 most common 2-grams.
    for count, bigram in bigram_counts_sort.take(10):
      print '%d:\t%s\t%s' % (count, bigram[0], bigram[1])

    bigram_counts.saveAsTextFile(gs_dest_full)

  if args.profile:
    sc.show_profiles()


def main():
  # Parse args.
  parser = argparse.ArgumentParser()
  parser.add_argument('--src', help='GS input dir.')
  parser.add_argument('--dst', help='GS output dir.')
  parser.add_argument('--year', default='*')
  parser.add_argument('--month', default='*')
  parser.add_argument('--day', default='*')
  parser.add_argument('--hour', default='*')
  parser.add_argument('--profile', default=False, action='store_true')
  probs = parser.add_mutually_exclusive_group(required=True)
  probs.add_argument(
      '--counts', dest='probs', action='store_false',
      help='Compute the number of occurrences of every bigram of tokens.')
  probs.add_argument('--probabilities', dest='probs', action='store_true',
      help=('Compute probability of any token being immediately followed by'
            ' another.'))
  parser.set_defaults(probs=False)
  args = parser.parse_args()
  if not args.src:
    raise Exception('--src is required.')
  if not args.dst:
    raise Exception('--dst is required.')

  process_logs(args)


if __name__ == '__main__':
  main()
