Google Cloud Datastore Migration Plan
=====================================

1. Write new store paths that use Datastore, controlled by a cmd line flag.
2. Write a migration tool that copies data from MySQL to Datastore.
3. For each instance of skiaperf:
    * Stop app.
    * Migrate data.
    * Flag flip on instance --use_cloud_datastore.
    * Restart app.
4. Update DESIGN.md to reflect the new way data is stored.
