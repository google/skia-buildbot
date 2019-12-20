import json
import sys

import subprocess

process = subprocess.Popen(["bot_config", "get_state"], stdin=subprocess.PIPE, stdout=subprocess.PIPE)

dim = {'foo': 'bar', 'baz': 123}
process.stdin.write(json.dumps(dim))
print json.loads(process.communicate()[0])
process.stdin.close()