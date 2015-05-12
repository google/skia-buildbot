# SQL for setting up the test database and user.
CREATE DATABASE IF NOT EXISTS sk_testing;
GRANT ALL ON sk_testing.* TO 'test_root'@'localhost';
GRANT SELECT,INSERT,UPDATE,DELETE ON sk_testing.* TO 'test_rw'@'localhost';

# SkiaPerf database.
CREATE DATABASE IF NOT EXISTS skia;
GRANT SELECT,INSERT,UPDATE,DELETE ON skia.* TO 'readwrite'@'localhost';

# SkiaGold database.
CREATE DATABASE IF NOT EXISTS skiacorrectness;
GRANT SELECT,INSERT,UPDATE,DELETE ON skiacorrectness.* TO 'readwrite'@'localhost';

# Buildbot database.
CREATE DATABASE IF NOT EXISTS buildbot;
GRANT SELECT,INSERT,UPDATE,DELETE ON buildbot.* TO 'readwrite'@'localhost';

# Alerts database.
CREATE DATABASE IF NOT EXISTS alerts;
GRANT SELECT,INSERT,UPDATE,DELETE ON alerts.* TO 'readwrite'@'localhost';

# Cluster Telemetry Frontend database.
CREATE DATABASE IF NOT EXISTS ctfe;
GRANT SELECT,INSERT,UPDATE,DELETE ON ctfe.* TO 'readwrite'@'localhost';
