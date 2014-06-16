cd /home/www-data/graphite/

export GRAPHITE_ROOT=/home/www-data/graphite
export PYTHONPATH=$PYTHONPATH:$GRAPHITE_ROOT/webapp:$GRAPHITE_ROOT/lib

# Create and/or update the database schema.
django-admin.py syncdb --settings=graphite.settings

# Start carbon.
$GRAPHITE_ROOT/bin/carbon-cache.py start
