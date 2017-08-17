import json
import os
import socket
import urllib
import urllib2


GS_URL_TMPL = 'https://www.googleapis.com/storage/v1/b/%s/o/%s?alt=media'
OAUTH_TOKEN = json.load(open('oauth_token.json', 'rb'))


def gs_download(bucket, src, dst):
  path = urllib.quote(src, safe='')
  url = GS_URL_TMPL % (bucket, path)
  req = urllib2.Request(url)
  req.add_header('Authorization', 'Bearer %s' % OAUTH_TOKEN['access_token'])
  print 'GET %s' % url
  resp = urllib2.urlopen(req)
  with open(dst, 'wb') as f:
    f.write(resp.read())
  print 'Wrote %s' % dst


home = os.path.expanduser('~')
netrc_base = '.netrc'
if os.name == 'nt':
  netrc_base = '_netrc'

user = 'bots'
if '-i-' in socket.gethostname():
  user = 'bots-internal'
downloads = {
  '.netrc_%s' % user: os.path.join(home, netrc_base),
  '.gitconfig': os.path.join(home, '.gitconfig'),
}
for src, dst in downloads.iteritems():
  src = '/'.join(('artifacts', 'bots', src))
  gs_download('skia-buildbots', src, dst)
  if 'netrc' in dst:
    os.chmod(dst, 0600)

