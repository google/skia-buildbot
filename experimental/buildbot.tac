import os

from twisted.application import service
from skia_master_scripts import db, skia_master

basedir = os.path.dirname(os.path.abspath(__file__))
configfile = r'master.cfg'

application = service.Application('buildmaster')
skia_master.SkiaMaster(basedir, configfile).setServiceParent(application)

