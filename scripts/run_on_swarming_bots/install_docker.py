#!/usr/bin/env python
#
# Copyright 2018 Google LLC
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""
Install Docker on a Swarming bot then "fail", causing a reboot. Aborts on error.
"""


import os
import subprocess
import sys

# This is a linux amd64 version of a credential helper recommended for use on
# the cloud. See
# https://cloud.google.com/container-registry/docs/advanced-authentication
AUTH_HELPER_URL = ('https://storage.googleapis.com/skia-backups/'
                   'docker-credential-gcr/1.5.0/docker-credential-gcr')

# Copied from
# https://skia.googlesource.com/buildbot/+show/d864d83d992f2968cf4d229cebf2d3104ee11ebf/go/gce/swarming/base-image/setup-script.sh#20
base_cmd = ['sudo', 'DEBIAN_FRONTEND=noninteractive', 'apt',
            '-o', 'quiet=2', '--assume-yes',
            '-o', 'Dpkg::Options::=--force-confdef',
            '-o', 'Dpkg::Options::=--force-confold']
# Install pre-reqs
subprocess.check_call(base_cmd + ['update'])
subprocess.check_call(base_cmd + [
    'install', 'apt-transport-https', 'ca-certificates', 'curl', 'gnupg2',
    'software-properties-common'])
# Set up docker repo
os.system(
    'curl -fsSL https://download.docker.com/linux/debian/gpg | '
    'sudo apt-key add -')
subprocess.check_call([
    'sudo', 'add-apt-repository',
    'deb [arch=amd64] https://download.docker.com/linux/debian stretch stable'])
# install docker
subprocess.check_call(base_cmd + ['update'])
subprocess.check_call(base_cmd + ['install', 'docker-ce'])
# Clear cache, which frees up 1G in many cases
subprocess.check_call(['sudo', 'apt-get', 'clean'])

# Stop the daemon and tell it to put the docker images in /mnt/pd0/docker, which
# has a bigger disk.
subprocess.check_call(['sudo', 'mkdir', '-p', '/mnt/pd0/docker'])
subprocess.check_call(['sudo', 'systemctl', 'stop', 'docker'])
os.system("sudo sh -c \"echo '{\\\"graph\\\":\\\"/mnt/pd0/docker\\\"}' >>"
          " /etc/docker/daemon.json\"")
subprocess.check_call(['sudo', 'systemctl', 'start', 'docker'])

# Test it out
subprocess.check_call(['sudo', 'docker', 'run', 'hello-world'])

# Add chrome-bot to docker group
# The following is necessary, but it appears installing docker now creates the
# group. Leaving this around mainly for documentation.
#subprocess.check_call(['sudo', 'groupadd', 'docker'])
subprocess.check_call(['sudo', 'usermod', '-aG', 'docker', 'chrome-bot'])

# Download the Docker authenticator helper, which will authenticate our calls to
# gcr.io properly
subprocess.check_call(['rm', '-f', '/home/chrome-bot/.docker/config.json'])
subprocess.check_call([
    'wget', '-O', '/tmp/docker-credential-gcr', AUTH_HELPER_URL])
subprocess.check_call(['chmod', '+x', '/tmp/docker-credential-gcr'])
subprocess.check_call(['sudo', 'cp', '/tmp/docker-credential-gcr', '/usr/bin'])
subprocess.check_call(['docker-credential-gcr', 'configure-docker'])

# Make docker run on startup
subprocess.check_call(['sudo', 'systemctl', 'enable', 'docker'])

print('Success! Marking the job failed so the bot reboots.')
# This will make the task say TASK_FAILED and reboot
sys.exit(1)
