# Attest Production Manual

General information about the service is available in the [README](./README.md).

# Alerts

## error_rate

An elevated error rate may indicate that permission to check attestations has
been lost, or the service is misconfigured. It may also be caused by an image
for which no attestation was generated being repeatedly checked, or by a user
(human or otherwise) making repeated invalid requests. Check the
[logs](https://pantheon.corp.google.com/logs/query;query=resource.labels.container_name%3D%attest%22?project=skia-infra-public)
and determine the root cause.
