# SQL for setting up the test database and user.
CREATE DATABASE IF NOT EXISTS sk_testing;
CREATE USER IF NOT EXISTS 'test_root'@'localhost' IDENTIFIED BY PASSWORD '';
CREATE USER IF NOT EXISTS 'test_rw'@'localhost' IDENTIFIED BY PASSWORD '';
GRANT ALL ON sk_testing.* TO 'test_root'@'localhost';
GRANT SELECT,INSERT,UPDATE,DELETE ON sk_testing.* TO 'test_rw'@'localhost';
FLUSH PRIVILEGES;
