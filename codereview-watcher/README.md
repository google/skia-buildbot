Codereview Watcher
==================

Skia (https://skia.googlesource.com/skia.git) is mirrored in Github
(https://github.com/google/skia). Skia sometimes get PRs in Github
([example](https://github.com/google/skia/pull/68)) and then have to ask
developers to re-upload in Gerrit.

Skia uses Copybara to automatically create a Gerrit change from a Github PR
(after CLA is signed). The Gerrit change created by Copybara is very hard to
find ([screenshot](https://screenshot.googleplex.com/6FU2sfCeZWPGA8i)).
The codereview-watcher framework attempts to fix that by updating the PR with a
descriptive message.

Cannot make Copybara config read Skia's rotation URLs (eg:
https://tree-status.skia.org/sheriff). So the codereview-watcher frameworks will
automatically set the sheriff (or trooper) as the review for changes created by
Copybara from Github PRs.

This framework is turned on for https://github.com/google/skia and
https://github.com/google/skia-buildbot.

Design doc is [here](http://goto/skia-github-gerrit).
