datahopper_internal
-------------------

Pulls data from the Android Build APIs and funnels that into the buildbot database.

Requires the following bits of project level Metadata:

  * datahopper_internal_targets - A space separated list of tradefed targets.
  * cookieSalt - Salt for cookie generation.
  * client_id - API Client ID.
  * client_secret - API Client Secret.
  * database_readwrite - Password for the build database read/write access.

The instance must also be setup with the scope
"https://www.googleapis.com/auth/androidbuild.internal" for the compute engine
service account.

