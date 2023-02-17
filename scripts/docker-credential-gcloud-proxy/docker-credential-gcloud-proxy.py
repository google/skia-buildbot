#!/usr/bin/env python3

# This is a helper script to debug Docker authorization issues.
#
# This script passes its stdin and command-line arguments to the docker-credential-gcloud program,
# captures its stdout, stderr and exit code, logs them in a file for debugging purposes, emits the
# captured stdout and stderr, and exits with the captured exit code.
#
# See usage instructions in README.md.

import datetime
import os
import subprocess
import sys

stdin = "\n".join(sys.stdin.readlines())

p = subprocess.run(["/usr/lib/google-cloud-sdk/bin/docker-credential-gcloud"] + sys.argv[1:], capture_output=True, text=True, input=stdin)

with open("/docker-credential-gcloud-proxy.log", "a") as f:
  f.write("===\n")
  f.write("time: %s\n" % str(datetime.datetime.now()))
  f.write("working directory: %s\n" % os.getcwd())
  f.write("environment variables:\n---\n%s\n---\n" % "\n".join(["%s=%s" % (v, os.environ[v]) for v in os.environ]))
  f.write("command: %s\n" % " ".join(sys.argv))
  f.write("exit code: %d\n" % p.returncode)
  f.write("stdin:\n---\n%s---\n" % stdin)
  f.write("stdout:\n---\n%s---\n" % p.stdout)
  f.write("stderr:\n---\n%s---\n" % p.stderr)

sys.stdout.write(p.stdout)
sys.stderr.write(p.stderr)
sys.exit(p.returncode)
