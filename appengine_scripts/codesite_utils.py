# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Utils for interacting with http://code.google.com."""


import re
import urllib
import urllib2


CODESITE_SKIA_CHANGES_URL = 'https://code.google.com/p/skia/source/list'
NUMBER_PARAM = 'num'
CHANGE_START_PARAM = 'start'


def GetNextRevNum():
  """Returns the next Skia revision number."""
  current_rev = GetCurrLatestRevNum()
  return current_rev + 1


def GetCurrLatestRevNum():
  """Returns the current latest Skia revision number."""
  connection = urllib2.urlopen('%s?%s=1' % (CODESITE_SKIA_CHANGES_URL,
                                            NUMBER_PARAM))
  try:
    page_content = connection.read()
  finally:
    connection.close()
  m = re.search('detail\?r\=(\d+)', page_content)
  return int(m.group(1))


def GetCodesiteUrlWithChangesRange(first_rev, last_rev=None):
  """Returns a URL that starts and ends at the specified changes.

  If the last revision number is not specified then the current latest Skia
  revision number is used.
  """
  if not first_rev:
    return None
  if not last_rev:
    last_rev = GetCurrLatestRevNum()
  if first_rev > last_rev:
    return None
  difference = last_rev - first_rev + 1
  params = {
    NUMBER_PARAM: difference,
    CHANGE_START_PARAM: last_rev,
  }
  return '%s?%s' % (CODESITE_SKIA_CHANGES_URL, urllib.urlencode(params))


if __name__ == '__main__':
  print '\nGetCurrLatestRevNum():'
  print GetCurrLatestRevNum()
  print '\nGetNextRevNum():'
  print GetNextRevNum()
  print '\nGetCodesiteUrlWithChangesRange(first_rev=7334, last_rev=7335):'
  print GetCodesiteUrlWithChangesRange(first_rev=7334, last_rev=7335)
  print '\nGetCodesiteUrlWithChangesRange(first_rev=7335, last_rev=7335):'
  print GetCodesiteUrlWithChangesRange(first_rev=7335, last_rev=7335)
  print '\nGetCodesiteUrlWithChangesRange(first_rev=7335, last_rev=7336):'
  print GetCodesiteUrlWithChangesRange(first_rev=7335, last_rev=7336)
  print '\nGetCodesiteUrlWithChangesRange(first_rev=7000, last_rev=7335)'
  print GetCodesiteUrlWithChangesRange(first_rev=7000, last_rev=7335)

