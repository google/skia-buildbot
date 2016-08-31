# Template for GAE Standard app

This directory contains "Hello, world" GAE Standard application
with all bells and whistles of luci-go framework:

*   HTTP router and middlewares.
*   luci/gae wrapper around datastore for better testability.
*   OpenID based authenticator layer.
*   Application settings store.


## Code structure

All luci-go code (including all GAE apps) is built **without** appengine tag,
since we support GAE Standard runtime as well as GAE Flexible runtime (and unit
testing environment running locally).

As a consequence, GAE apps must live in valid Go packages, in particular they
must not use relative import paths (as advertised by examples for Standard SDK).

Since we still want to run on Standard GAE, we must obey some rules.
In particular, Standard GAE Golang SDK enforces that all subpackages imported
from GAE module package (e.g. package that contains `app.yaml`) are using
relative import paths. For example, if `app.yaml` is located in the package
`github.com/luci/luci-go/luci-go/appengine/cmd/helloworld` then an attempt to
import `github.com/luci/luci-go/luci-go/appengine/cmd/helloworld/subpackage`
from `.../helloworld/source.go` will fail with GAE SDK complaining that imports
should be relative (i.e. `import "subpackage"`). It is a precaution against
importing instances of same package via two different import paths (absolute and
relative), since in this case all types, init() calls etc are duplicated. More
info is [here](https://groups.google.com/forum/#!topic/google-appengine-go/dNhqV6PBqVc).

Luckily, Standard GAE SDK handles imports from packages not under module root
in a same way as normal Golang SDK does. To workaround the weird limitation
mentioned above, we put `app.yaml` (and HTTP routing code) in a separate
`frontend` package, that doesn't have any subpackages.

To summarize, the structure of GAE app is:

*   `.../cmd/app/frontend` contains `app.yaml` and code to setup HTTP routes for
    default GAE module. It can also contain `cron.yaml`, `queues.yaml` and other
    GAE yamls. It must not have subpackages.
*   `.../cmd/app/backend` contains `module-backend.yaml` that defines `backend`
    GAE module and code to setup HTTP routes for backend module. It must not
    have subpackages.
*   `.../cmd/app/logic1`, `.../cmd/app/logic2`, ... contains actual
    implementation of app's logic. It is normal Go packages that can have
    subpackages, tests, etc.


## Running on devserver

The simplest way to run the app locally is to use `gae.py` wrapper around
appcfg from [luci-py](https://github.com/luci/luci-py) repository.

Clone luci-py somewhere and symlink
[gae.py](https://github.com/luci/luci-py/blob/master/appengine/components/tools/gae.py)
as `gae.py` in GAE application directory (i.e. a directory that contains
`frontend` directory as a child).

If you are using infra.git gclient solution, run:

```shell
cd github.com/luci/luci-go/luci-go/appengine/cmd/helloworld
ln -s ../../../../../../../../luci/appengine/components/tools/gae.py gae.py
./gae.py devserver
```

## Deploying and configuring

Use `gae.py` to deploy:

```shell
./gae.py upload -A <appid>
./gae.py switch -A <appid>
```

When deployed for a first time you'd need to configure OAuth2 client (used for
OpenID login flow) and URL of an auth service to grab user groups from.

To configure OAuth2:

1.  Go to [Cloud Console](https://console.developers.google.com) for some
    project of your choosing (not necessarily same one that GAE app belongs to).
1.  On "Credentials" tab of "API Manager" service create a new web application
    OAuth 2.0 client ID (using "Add Credentials" button and choosing "Web
    application" type).
1.  In "Authorized redirect URIs" add
    `https://<yourapp>.appspot.com/auth/openid/callback`.
1.  As a result you'll get client ID and client secret strings. Note them.
1.  Go to `https://<yourapp>.appspot.com/admin/settings/openid_auth`. You must
    be GAE level administrator of the app to access this page.
1.  In "Discovery URL" field enter `https://accounts.google.com/.well-known/openid-configuration`
1.  In "OAuth client ID", "OAuth client secret" and "Redirect URI" fields put
    values you used when configuring client ID in the Cloud Console.
1.  Click "Save settings". One minute later OpenID authentication should start
    working.

It is possible to reuse existing OAuth 2.0 web clients. Just add a new redirect
URI to the list of authorized redirect URIs.

To start using auth groups from some existing auth service
(e.g. [chrome-infra-auth.appspot.com](https://chrome-infra-auth.appspot.com)):

1.  Go to [Cloud Console](https://console.developers.google.com) project that
    contain your GAE app.
1.  In "API Manager" section enable "Google Cloud Pub/Sub" API.
1.  Go to `https://<yourapp>.appspot.com/admin/settings/auth_service`.
1.  Note service account ID corresponding to your GAE app. It is highlighted in
    bold in "Authorization settings" section.
1.  Add this account to `auth-trusted-services` group on the auth service. You
    must be an admin on auth server to be able to do so.
1.  Put URL of the auth server into "Auth service URL" field, click
    "Save settings".
