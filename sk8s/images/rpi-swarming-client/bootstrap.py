import json
import sys
import urllib2

# See RPI_DESIGN.md.
token_url = ('http://metadata:8000/computeMetadata/v1/instance/'
             'service-accounts/default/token')
req = urllib2.Request(token_url, headers={'Metadata-Flavor': 'Google'})
tok = json.load(urllib2.urlopen(req))
req = urllib2.Request(sys.argv[1] + '/bootstrap',
  headers={'Authorization': 'Bearer %s' % tok['access_token']})
exec urllib2.urlopen(req).read()
