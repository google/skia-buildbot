#!/usr/bin/env python
# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# Tool for seeing the real world impact of a patch.
#
# Layout Tests can tell you whether something has changed, but this can help
# you determine whether a subtle/controversial change is beneficial or not.
#
# It dumps the rendering of a large number of sites, both with and without a
# patch being evaluated, then sorts them by greatest difference in rendering,
# such that a human reviewer can quickly review the most impacted sites,
# rather than having to manually try sites to see if anything changes.
#
# In future it might be possible to extend this to other kinds of differences,
# e.g. page load times.
#
# pylint: disable=C0301
# The original file is from http://src.chromium.org/viewvc/chrome/trunk/src/tools/real_world_impact/real_world_impact.py
# It was written by johnme@ and modified by pdr@.
# rmistry@ has renamed the file and modified it to run on the Cluster telemetry
# 100 slaves (http://skia-tree-status-staging.appspot.com/skia-telemetry/chromium_try).

import argparse
from argparse import RawTextHelpFormatter
import datetime
import errno
from distutils.spawn import find_executable
from operator import itemgetter
import multiprocessing
import os
import posixpath
import re
import subprocess
import sys
import textwrap
import time
from urlparse import urlparse
import webbrowser


action = None
allow_js = False
additional_content_shell_flags = ''
output_dir = ''
image_diff = ''
content_shell = ''
urls = []
print_lock = multiprocessing.Lock()


def MakeDirsIfNotExist(directory):
  try:
    os.makedirs(directory)
  except OSError as e:
    if e.errno != errno.EEXIST:
      raise


def SetupPaths():
  MakeDirsIfNotExist(output_dir)
  return True


def CheckPrerequisites():
  if not find_executable('wget'):
    print 'wget not found! Install wget and re-run this.'
    return False
  if not os.path.exists(image_diff):
    print 'image_diff not found (%s)!' % image_diff
    print 'Build the image_diff target and re-run this.'
    return False
  if not os.path.exists(content_shell):
    print 'Content shell not found (%s)!' % content_shell
    print 'Build Release/content_shell and re-run this.'
    return False
  return True


def PickSampleUrls(start_number, end_number, csv_path):
  global urls
  data_dir = os.path.join(output_dir, 'data')
  MakeDirsIfNotExist(data_dir)

  bad_urls_path = os.path.join(data_dir, 'bad_urls.txt')
  if os.path.exists(bad_urls_path):
    with open(bad_urls_path) as f:
      bad_urls = set(f.read().splitlines())
  else:
    bad_urls = set()

  # See if we've already selected the same sample previously (this way, if you
  # call this script with arguments
  # '--start_number=1 --end_number=10 --action=before' then
  # '--start_number=1 --end_number=10 --action=after', we'll use the same
  # sample, as expected!).
  urls_path = os.path.join(data_dir, '%d-%d_urls.txt' % (start_number,
                                                         end_number))
  if not os.path.exists(urls_path):
    if action == 'compare':
      print ('Error: you must run "--action=before" and "--action=after" '
             'before running "--action=compare"')
      return False
    print 'Picking %d-%d from the Alexa list...' % (start_number, end_number)

    urls = []
    current_rank = 0
    with open(csv_path) as f:
      for entry in f:
        current_rank += 1
        if current_rank < start_number:
          continue
        elif current_rank > end_number:
          break
        hostname = entry.strip().split(',')[1]
        if not '/' in hostname:  # Skip Alexa 1,000,000 entries that have paths.
          url = 'http://%s/' % hostname
          if not url in bad_urls:
            urls.append(url)
    # Don't write these to disk yet; we'll do that in SaveWorkingUrls below
    # once we have tried to download them and seen which ones fail.
  else:
    with open(urls_path) as f:
      urls = [u for u in f.read().splitlines() if not u in bad_urls]
  return True


def SaveWorkingUrls(start_number, end_number):
  # TODO(johnme): Update the list if a url that used to work goes offline.
  urls_path = os.path.join(output_dir, 'data', '%d-%d_urls.txt' % (start_number,
                                                                   end_number))
  if not os.path.exists(urls_path):
    with open(urls_path, 'w') as f:
      f.writelines(u + '\n' for u in urls)


def PrintElapsedTime(elapsed, detail=''):
  elapsed = round(elapsed * 10) / 10.0
  m = elapsed / 60
  s = elapsed % 60
  print 'Took %dm%.1fs' % (m, s), detail


def DownloadStaticCopyTask(url):
  url_parts = urlparse(url)
  host_dir = os.path.join(output_dir, 'data', url_parts.hostname)
  # Use wget for now, as does a reasonable job of spidering page dependencies
  # (e.g. CSS, JS, images).
  success = True
  try:
    subprocess.check_call(['timeout', '60',
                           'wget',
                           '--execute', 'robots=off',
                           ('--user-agent=Mozilla/5.0 (Macintosh; Intel Mac OS '
                            'X 10_8_5) AppleWebKit/537.36 (KHTML, like Gecko) C'
                            'hrome/32.0.1700.14 Safari/537.36'),
                           '--page-requisites',
                           '--span-hosts',
                           '--adjust-extension',
                           '--convert-links',
                           '--directory-prefix=' + host_dir,
                           '--force-directories',
                           '--default-page=index.html',
                           '--no-check-certificate',
                           '--timeout=5', # 5s timeout
                           '--tries=2',
                           '--quiet',
                           url])
  except KeyboardInterrupt:
    success = False
  except subprocess.CalledProcessError:
    # Ignoring these for now, as some sites have issues with their subresources
    # yet still produce a renderable index.html
    pass #success = False
  if success:
    download_path = os.path.join(host_dir, url_parts.hostname, 'index.html')
    if not os.path.exists(download_path):
      success = False
    else:
      with print_lock:
        print 'Downloaded:', url
  if not success:
    with print_lock:
      print 'Failed to download:', url
    return False
  return True


def DownloadStaticCopies(start_number, end_number):
  global urls
  new_urls = []
  for url in urls:
    url_parts = urlparse(url)
    host_dir = os.path.join(output_dir, 'data', url_parts.hostname)
    download_path = os.path.join(host_dir, url_parts.hostname, 'index.html')
    if not os.path.exists(download_path):
      new_urls.append(url)

  if new_urls:
    print 'Downloading static copies of %d sites...' % len(new_urls)
    start_time = time.time()

    results = multiprocessing.Pool(20).map(DownloadStaticCopyTask, new_urls)
    failed_urls = [new_urls[i] for i, ret in enumerate(results) if not ret]
    if failed_urls:
      bad_urls_path = os.path.join(output_dir, 'data', 'bad_urls.txt')
      with open(bad_urls_path, 'a') as f:
        f.writelines(u + '\n' for u in failed_urls)
      failed_urls_set = set(failed_urls)
      urls = [u for u in urls if u not in failed_urls_set]

    PrintElapsedTime(time.time() - start_time)

  SaveWorkingUrls(start_number, end_number)


def RunDrtTask(url):
  url_parts = urlparse(url)
  host_dir = os.path.join(output_dir, 'data', url_parts.hostname)
  html_path = os.path.join(host_dir, url_parts.hostname, 'index.html')

  if not allow_js:
    nojs_path = os.path.join(host_dir, url_parts.hostname, 'index-nojs.html')
    if not os.path.exists(nojs_path):
      with open(html_path) as f:
        html = f.read()
      if not html:
        return False
      # These aren't intended to be XSS safe :)
      block_tags = (r'<\s*(script|object|video|audio|iframe|frameset|frame)'
                    r'\b.*?<\s*\/\s*\1\s*>')
      block_attrs = r'\s(onload|onerror)\s*=\s*(\'[^\']*\'|"[^"]*|\S*)'
      html = re.sub(block_tags, '', html, flags=re.I|re.S)
      html = re.sub(block_attrs, '', html, flags=re.I)
      with open(nojs_path, 'w') as f:
        f.write(html)
    html_path = nojs_path

  start_time = time.time()

  with open(os.devnull, 'w') as fnull:
    dump_tree_cmd = [content_shell,
                     '--dump-render-tree',
                     additional_content_shell_flags,
                     # The escaped single quote is not a typo, it's a separator!
                     html_path + "\\'--pixel-test"
                    ]
    p = subprocess.Popen(' '.join(dump_tree_cmd),
                         shell=True,
                         stdout=subprocess.PIPE,
                         stderr=fnull)
  result = p.stdout.read()

  PNG_START = b'\x89\x50\x4E\x47\x0D\x0A\x1A\x0A'
  PNG_END = b'\x49\x45\x4E\x44\xAE\x42\x60\x82'
  try:
    start = result.index(PNG_START)
    end = result.rindex(PNG_END) + 8
  except ValueError:
    return False

  png_path = os.path.join(output_dir, action, url_parts.hostname + '.png')
  MakeDirsIfNotExist(os.path.dirname(png_path))
  with open(png_path, 'wb') as f:
    f.write(result[start:end])
  elapsed_time = (time.time() - start_time, url)
  return elapsed_time


def RunDrt():
  print 'Taking screenshots of %d pages...' % len(urls)
  start_time = time.time()

  results = multiprocessing.Pool().map(RunDrtTask, urls, 1)

  max_time, url = max(t for t in results if t)
  elapsed_detail = '(slowest: %.2fs on %s)' % (max_time, url)
  PrintElapsedTime(time.time() - start_time, elapsed_detail)


def CompareResultsTask(url):
  url_parts = urlparse(url)
  before_path = os.path.join(output_dir, 'before', url_parts.hostname + '.png')
  after_path = os.path.join(output_dir, 'after', url_parts.hostname + '.png')
  diff_path = os.path.join(output_dir, 'diff', url_parts.hostname + '.png')
  MakeDirsIfNotExist(os.path.join(output_dir, 'diff'))

  red_path = ('data:image/gif;base64,R0lGODlhAQABAPAAAP8AAP///yH5BAAAAAAALAAAAA'
              'ABAAEAAAICRAEAOw==')

  before_exists = os.path.exists(before_path)
  after_exists = os.path.exists(after_path)
  if not before_exists and not after_exists:
    # TODO(johnme): Make this more informative.
    return (-100, url, red_path)
  if before_exists != after_exists:
    # TODO(johnme): Make this more informative.
    return (200, url, red_path)

  # Get percentage difference.
  p = subprocess.Popen([image_diff, '--histogram',
                        before_path, after_path],
                        shell=False,
                        stdout=subprocess.PIPE)
  output, _ = p.communicate()
  if p.returncode == 0:
    return (0, url, before_path)
  diff_match = re.match(r'histogram diff: (\d+\.\d{2})% (?:passed|failed)\n'
                         'exact diff: (\d+\.\d{2})% (?:passed|failed)', output)
  if not diff_match:
    raise Exception('image_diff output format changed')
  histogram_diff = float(diff_match.group(1))
  exact_diff = float(diff_match.group(2))
  combined_diff = max(histogram_diff + exact_diff / 8, 0.001)

  # Produce diff PNG.
  subprocess.call([image_diff, '--diff', before_path, after_path, diff_path])
  return (combined_diff, url, diff_path)


def CompareResults(start_number, end_number, gs_url_prefix):
  print 'Running image_diff on %d pages...' % len(urls)
  start_time = time.time()

  results = multiprocessing.Pool().map(CompareResultsTask, urls)
  results.sort(key=itemgetter(0), reverse=True)

  PrintElapsedTime(time.time() - start_time)

  now = datetime.datetime.today().strftime('%a %Y-%m-%d %H:%M')
  html_start = textwrap.dedent("""\
  <!DOCTYPE html>
  <html>
  <head>
  <title>Real World Impact report %s</title>
  <script>
    var togglingImg = null;
    var toggleTimer = null;

    var before = true;
    function toggle() {
      var newFolder = before ? "\/before" : "\/after";
      togglingImg.src = togglingImg.src.replace(/\/before|\/after|\/diff/, newFolder);
      before = !before;
      toggleTimer = setTimeout(toggle, 300);
    }

    function startToggle(img) {
      before = true;
      togglingImg = img;
      if (!img.origSrc)
        img.origSrc = img.src;
      toggle();
    }
    function stopToggle(img) {
      clearTimeout(toggleTimer);
      img.src = img.origSrc;
    }

    document.onkeydown = function(e) {
      e = e || window.event;
      var keyCode = e.keyCode || e.which;
      var newFolder;
      switch (keyCode) {
        case 49: //'1'
          newFolder = "\/before"; break;
        case 50: //'2'
          newFolder = "\/after"; break;
        case 51: //'3'
          newFolder = "\/diff"; break;
        default:
          return;
      }
      var imgs = document.getElementsByTagName("img");
      for (var i = 0; i < imgs.length; i++) {
        imgs[i].src = imgs[i].src.replace(/\/before|\/after|\/diff/, newFolder);
      }
    };
  </script>
  <style>
    h1 {
      font-family: sans;
    }
    h2 {
      font-family: monospace;
      white-space: pre;
    }
    .nsfw-spacer {
      height: 50vh;
    }
    .nsfw-warning {
      background: yellow;
      border: 10px solid red;
    }
    .info {
      font-size: 1.2em;
      font-style: italic;
    }
    body:not(.details-supported) details {
      display: none;
    }
  </style>
  </head>
  <body>
    <script>
    if ('open' in document.createElement('details'))
      document.body.className = "details-supported";
    </script>
    <!--<div class="nsfw-spacer"></div>-->
    <p class="nsfw-warning">Warning: sites below are taken from the Alexa
    top %d-%d and may be NSFW.</p>
    <!--<div class="nsfw-spacer"></div>-->
    <h1>Real World Impact report %s</h1>
    <p class="info">Press 1, 2 and 3 to switch between before, after and diff
    screenshots respectively; or hover over the images to rapidly alternate
    between before and after.</p>
  """ % (now, start_number, end_number, now))

  html_same_row = """\
  <h2>No difference on <a href="%s">%s</a>.</h2>
  """

  html_diff_row = """\
  <h2>%7.3f%% difference on <a href="%s">%s</a>:</h2>
  <img src="%s" width="800" height="600"
       onmouseover="startToggle(this)" onmouseout="stopToggle(this)">
  """

  html_end = textwrap.dedent("""\
  </body>
  </html>""")

  html_path = os.path.join(output_dir, 'diff.html')
  with open(html_path, 'w') as f:
    f.write(html_start)
    for (diff_float, url, diff_path) in results:
      diff_path = os.path.relpath(diff_path, output_dir)
      if diff_float == 0:
        f.write(html_same_row % (url, url))
      else:
        f.write(html_diff_row % (
            diff_float, url, url, posixpath.join(gs_url_prefix, diff_path)))
    f.write(html_end)

  webbrowser.open_new_tab('file://' + html_path)


def main(argv):
  global action, allow_js, output_dir, additional_content_shell_flags, \
         image_diff, content_shell

  parser = argparse.ArgumentParser(
      formatter_class=RawTextHelpFormatter,
      description='Compare the real world impact of a content shell change.',
      epilog=textwrap.dedent("""\
          Example usage:
            1. Build content_shell in out/Release without any changes.
            2. Run: %s --action=before --start_number=1 --end_number=10
            3. Either:
                 a. Apply your controversial patch and rebuild content_shell.
                 b. Pass --additional_flags="--enable_your_flag" in step 4.
            4. Run: %s --action=after --start_number=1 --end_number=10
            5. Run: %s --action=compare --start_number=1 --end_number=10
          """ % (argv[0], argv[0], argv[0])))
  parser.add_argument('--allow_js', help='Do not disable Javascript',
                      action='store_true')
  parser.add_argument('--additional_flags',
                      help='Additional flags to pass to content shell')
  parser.add_argument('--action',
                      help=textwrap.dedent("""\
                        Action to perform.
                          download - Just download the sites.
                          before - Run content shell and record 'before' result.
                          after - Run content shell and record 'after' result.
                          compare - Compare before and after results.
                      """),
                      choices=['download', 'before', 'after', 'compare'],
                      required=True)
  parser.add_argument('--start_number',
                      help='Specifies which website rank (in Alexa\'s list) to '
                           'start with',
                      type=int, required=True)
  parser.add_argument('--end_number',
                      help='Specifies which website rank (in Alexa\'s list) to '
                           'end with',
                      type=int, required=True)
  parser.add_argument('--output_dir',
                      help='Directory where output files will be stored',
                      required=True)
  parser.add_argument('--csv_path',
                      help='Path to the Alexa top 1M webpages CSV',
                      required=True)
  parser.add_argument('--chromium_out_dir',
                      help='Path to Chromium build\'s out directory.',
                      required=True)
  parser.add_argument('--gs_url_prefix',
                      help='The GS prefix to use which points to img files.',
                      required=True)

  args = parser.parse_args()

  action = args.action
  output_dir = os.path.join(args.output_dir, 'real_world_impact')
  gs_url_prefix = args.gs_url_prefix
  chromium_out_dir = args.chromium_out_dir
  image_diff = os.path.join(chromium_out_dir, 'image_diff')
  content_shell = os.path.join(chromium_out_dir, 'content_shell')
  csv_path = args.csv_path
  start_number = args.start_number
  end_number = args.end_number

  if (args.allow_js):
    allow_js = args.allow_js

  if (args.additional_flags):
    additional_content_shell_flags = args.additional_flags

  if not SetupPaths() or not CheckPrerequisites() or not PickSampleUrls(
      start_number, end_number, csv_path):
    return 1

  if action == 'compare':
    CompareResults(start_number, end_number, gs_url_prefix)
  else:
    DownloadStaticCopies(start_number, end_number)
    if action != 'download':
      RunDrt()
  return 0


if __name__ == '__main__':
  sys.exit(main(sys.argv))
