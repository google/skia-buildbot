# Scrap Exchange

A unified backend service for storing scraps of code across all of Skia Infrastructure's services.

## Overview

Skia Infrastructure currently has services that store scraps of code, such as
https://shaders.skia.org. The term 'scrap' implies that the code can not stand
on its own, such as the JSON that underlies SkSL shaders, or JS demos.

Scrap Exchange is a unified backend service that is constructed to make it
easier to stand up such web UIs, and to also make linking top level web UIs
together easier, for example, going from a particle 'scrap' on
shaders.skia.org to complete C++ code on fiddle.skia.org, or complete JS code
on jsfiddle.skia.org, that runs the shaders. The scrap service has the
following features:

- Store scraps by 'type', i.e. SVG vs SkSL vs C++.
- Load scraps via URL.
- Store meta-data with scraps. For example, one SkSL scrap may refer to other
  child shaders, so the meta-data will contain the references to those other
  scraps. Other things that could be stored in metadata include the values of
  controls that are used as inputs to a shader.
- Ability for Infrastructure team members to review and delete scraps as
  needed, such as copyright violations.
- The scrap server will also allow 'wrapping' scraps in boilerplate code. For
  example, an SkSL scrap can be wrapped in C++ and then used directly by
  fiddle. This includes understanding the metadata about the scrap, such as
  child shaders, and filling in the C++ code template appropriately.

See http://go/scrap-exchange for more details.

## Auth

Uses the `skia-public-auth@skia-public.iam.gserviceaccount.com` service account,
originally created in `//infra/am`, to talk to the Chrome Infra Auth API.

## Buckets

We use the following buckets for each instance:

    skia-public: gs://skia-public-scrap-exchange

## Deployment

Scrap Exchange is automatically built by a
[Louhi flow](https://louhi.dev/6316342352543744/flow-detail/dee997a6-0306-49f9-b902-dd8a7e7aab9b?branch=main)
whenever a change merges anywhere in this repository. If this results in a new
Docker image in
[gcr.io/skia-public/scrapexchange](https://console.cloud.google.com/gcr/images/skia-public/global/scrapexchange)
then Louhi will automatically update references in k8s-config whereby
k8s-deployer will automatically deploy that new image. Deployment is fully
automatic.

A Docker image can be manually pushed to GCR by running
`make push_I_am_really_sure`. In addition to pushing, this target will also
update k8s-config references. This will prevent Louhi from updating future
scrapexchange references in k8s-config. To re-enable updating, a manual change
in k8s-config will need to be landed that returns the image reference from
tag-style (e.g. `gcr.io/skia-public/scrapexchange:{tagname}`)
to digest style (e.g. `cr.io/skia-public/scrapexchange@sha256:{digest}`).
