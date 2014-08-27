cd /home/www-data
pwd

# Install local (for www-data) versions of carbon and its dependencies.
pip install https://github.com/graphite-project/ceres/tarball/master \
  --install-option="--prefix=/home/www-data/graphite" \
  --install-option="--install-lib=/home/www-data/graphite/lib"
pip install whisper --install-option="--prefix=/home/www-data/graphite" \
  --install-option="--install-lib=/home/www-data/graphite/lib"
pip install carbon --install-option="--prefix=/home/www-data/graphite" \
  --install-option="--install-lib=/home/www-data/graphite/lib"

# Install graphite-web.
if [ -d graphite-web ]; then
  (cd graphite-web && git pull);
else
  git clone https://github.com/graphite-project/graphite-web.git
fi

cd graphite-web
python setup.py  install --prefix=/home/www-data/graphite \
  --install-lib=/home/www-data/graphite/lib

HOME=/home/www-data
cd $HOME
# Install Go
if [ -d go ]; then
  echo Go already installed.
else
  wget https://go.googlecode.com/files/go1.2.1.linux-amd64.tar.gz
  tar -xzf go1.2.1.linux-amd64.tar.gz
fi

mkdir=$HOME/golib
# Prebuilt Go binaries default to /usr/local/go
export GOROOT=$HOME/go
export GOPATH=$HOME/golib
export PATH=$PATH:$GOROOT/bin

# Get buildbot code so we can build the prober.
if [ -d buildbot ]; then
  (cd buildbot && git pull && git checkout tags/0.9.12);
else
  git clone https://skia.googlesource.com/buildbot
fi

cd buildbot/compute_engine_scripts/monitoring/prober
go get -d
go build
