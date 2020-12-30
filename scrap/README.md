# Scrap Exchange

A unified backend service for storing scraps of code across all of Skia Infrastructure's services.

## Overview

Skia Infrastructure currently has services that store scraps of code, such as
https://particles.skia.org. The term 'scrap' implies that the code can not stand
on its own, such as the JSON that underlies particles, or SkSL shaders.

Scrap Exchange is a unified backend service that is constructed to make it
easier to stand up such web UIs, and to also make linking top level web UIs
together easier, for example, going from a particle 'scrap' on
particles.skia.org to complete C++ code on fiddle.skia.org, or complete JS code
on jsfiddle.skia.org, that runs the particles. The scrap service has the
following features:

- Store scraps by 'type', i.e. SVG vs SkSL vs Particles.
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
