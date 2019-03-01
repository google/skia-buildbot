import json
import urllib

AUTH_URL = ('http://metadata/computeMetadata/v1/instance'
            '/service-accounts/default/token')
response = urllib.urlopen(AUTH_URL)
auth = json.loads(response.read())
print auth.access_token