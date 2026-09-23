# Metadata Server

This program emulates the metadata server in GCE and is intended to be used in
the Skolo just for serving tokens to swarming clients. Note that the serving
path is hard-coded to
/computeMetadata/v1/instance/service-accounts/default/token.

**WARNING:** This service runs a web server which provides unrestricted access
to the service account whose key is baked into it. The compiled binary should be
considered a secret and treated with the same care as the service account key
itself. Further, it is imperative that the service is never exposed beyond the
network in which the test machines are running. It is also strongly recommended
that the service account is only used for identity, ie. authenticating to
Swarming, and has no other permissions. In the event that this service is ever
compromised, the service account key should be rotated immediately.
