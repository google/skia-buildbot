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
