# Template for multi-module GAE Flexible app

It is mostly identical to GAE Standard app and can be treated in exact same
way (i.e. via `gae.py`).

The differences are:

  * Do not put `application` and `version` in YAMLs. MVMs do not like them.
    Always use `-A` (e.g. `gae.py upload -A <app-id>`) when uploading the app
    instead.
  * Always use `package main` for packages that contain module YAMLs.
  * Use `func main()` (calling `appengine.Main()`) instead of `func init()`.
