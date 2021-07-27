# SkCQ Production Manual

General information about SkCQ is available in the [README](./README.md).

## Diagnosing SkCQ errors for a CL

When SkCQ has an unrecoverable error when processing a CL it will remove that CL
from the SkCQ and post a comment that looks like:
"Error when ${action}. Removing from CQ. Please ask Infra Gardener to investigate"
([example](https://skia-review.googlesource.com/c/skia/+/433180/1#message-85e2132acda4e84badaf5aff6e04162184e94ab9)).

To find the reason, search in skcq-be logs [here](https://pantheon.corp.google.com/logs/viewer?project=google.com:skia-corp&minLogLevel=500&resource=container&folder&organizationId=433637338589&expandAll=false&logName=projects%2Fgoogle.com:skia-corp%2Flogs%2Fskcq-be) for the CL number.

## How to rollback the Skia repo from SkCQ to ChromeCQ

Documented [here](https://docs.google.com/document/d/1x0K09xD_dtQnQ_WCJ3xYsJj64rVqybKAa5VBq-ZRi5E/edit?resourcekey=0-xbkxsZ1l5_XpH0PYIqulcg#heading=h.pvawa4dpv8hc).
