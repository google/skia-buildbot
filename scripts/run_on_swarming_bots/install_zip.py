import subprocess

# This installs zip by downloading the debian directly instead of using apt-get.
# This can get around apt-get update not working due to the Debian version
# moving from stable to oldstable.
ZIP_URL = 'https://ftp.debian.org/debian/pool/main/z/zip/zip_3.0-12_amd64.deb'
subprocess.check_call(['wget', ZIP_URL, '--output-document=temp.deb'])
subprocess.check_call(['sudo', 'dpkg', '--install', 'temp.deb'])