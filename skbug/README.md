# skbug.com

This uses Firebase hosting to server skbug.com, which is a simple redirector to
https://bugs.skia.org, which is the official redirector to the Skia issue
tracker.

Firebase also handles the SSL cert for skbug.com.

## Deploying

Make sure you run `firebase login` before running `make push`.