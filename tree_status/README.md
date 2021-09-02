Skia Tree Status
================

Design doc for multi-repo support is [here](http://go/skia-multiple-tree-statuses).

How to add support for a repo to tree status
--------------------------------------------

1. Add a `--repo=${repo}` flag to tree-status.yaml for public repos or

   tree-status-internal.yaml for private repos and push.


2. If the repo has SkCQ, then add to it's `infra/skcq.json` file:

   For public repo-

   `"tree_status_url": "https://tree-status.skia.org/${repo}/current"`

   For private repo-

   `"tree_status_url": "http://tree-status-internal:8001/${repo}/current"`


3. If the repo has a status page, then it's tree status should automatically

   show up on the status page after step 1. This is possible due to the work in

   [skbug/12394](https://skbug.com/12394).


4. The skiastatus plugin on Gerrit will automatically show the repo's tree

   status after step1. This is possible due to the work in

   [skbug/12395](https://skbug.com/12395).
