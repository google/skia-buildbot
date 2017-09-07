Google Cloud Datastore Migration Plan
=====================================

1. Write new store paths that use Datastore, controlled by a cmd line flag.
2. Write a migration tool that copies data from MySQL to Datastore.
3. Take down each instance, migrate data, flag flip on instance, restart.
