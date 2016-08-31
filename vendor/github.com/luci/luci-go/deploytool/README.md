The LUCI cloud services deployment tool
=======================================

The LUCI project hosts code on Google AppEngine, Google Container Engine (via
[Kubernetes](http://kubernetes.io/)), and several other locations. The code
layout is complex and modular, which is oftentimes at odds with the layout
requirements of supported deployment tools such as `gcloud` and `docker`. The
`luci_deploy` attempts to smooth that over by providing:

* A common declarative deployment configuration language.
* Hermetic, pinned source checkouts.
* A single tool capable of organizing and executing deployment programs.
* A utility that facilitates deployment management given its understanding of
  the deployment parameters.

While much of the underlying code in the deployment tool is cross-platform, it
is specifically targeting Linux support. Any deployment from other platforms is,
at this time, unsupported.

The deployment tool doesn't reimplement all of the functionality of existing
tools; instead, it manages configurations and layouts and employs existing
tooling to perform the actual deployment operations. Some tools that it uses
are:
* [gcloud](https://cloud.google.com/sdk/gcloud/), the Google Cloud SDK tool.
* [kubectl](http://kubernetes.io/docs/user-guide/kubectl-overview/), the
  Kubernetes control tool.
* [aedeploy](https://godoc.org/google.golang.org/appengine/cmd/aedeploy),
  a tool which collects `GOPATH` packages for Docker deployment.
* [docker](https://www.docker.com/), a container management system used by
  Kubernetes.

Deployment configuration for a given deployable component is stored in two
places:

1. Generic component configuration and requirements are stored alongside the
   component in its source repository.
1. Deployment-specific parameters are stored in a separate repository, and
   project the generic component layout into a product space.

The component-specific configuration details which resources the component
needs, which services it employs, and its relationship with other components.
The deployment projection then takes that generic configuration and applies it
to a set of specific deployment parameters. For example:

* One deployment may project a component into a production environment,
  allocating expensive CPU resources.
* A development deployment may project the same component into a development
  environment.
* A staging deployment may project the same component into a staging
  enviornment alongside other components from other projects.


## Overview and Terminology

Deployment configuration uses the following concepts:

* A **source** is a specific named repository checked out at a specific
  revision.
* A **source group** is a collection of **source** repositories.
* A **component** is a single buildable and deployable entity. Its configuration
  resides in a specific **source**.
* An **application** is a collection of deployable **component** pointers. Each
  pointer is a subpath to the **component** configuration within its **source**.
* A **resource** is a global cloud resource (e.g., cloud project, Kubernetes
  cluster, etc.).
* A **deployment** binds an **application** to a **source group** (and,
  consequently, a specific set of **sources**) and a set of **resources**.

During deployment, several operations are performed:

* The **working directory** is used to contain files generated and managed by
  the `luci_deploy`.
* The **checkout** is a read-only directory containing all of the **sources**
  checked out at their specific revisions and initialized.
* The **staging** directory is a constructed environment containing deployment
  and component sources and generated files organized such that deployment tools
  can operate on them. See [Staging](#staging) for more information.

A user will first perform a `checkout`, which loads all of the configured
**sources** at their specified revisions into the working directory. Afterwards,
the user performs operations on the checkout:
* `deploy`, which deploy the configured **deployments** and their
  composite **components**.
* `manage`, which offers component management utilities based on the
  deployment configuration.


## Deployment Layout

The deployment layout is a filesystem-based configuration structure that
defines the parameters of the configured deployments. Unless stated otherwise,
all layout component names may only contain:
- Alphanumeric characters (`[0-9a-zA-Z]`)
- A hyphen (`-`).

Configuration files are text-encoded protobufs. All protobufs used by the
`luci_deploy` are defined in its [api directory](api/deploy). Text protobuf
configuration files have the ".cfg" suffix. The protobuf messages used in the
layout are defined in [config.proto](api/deploy/config.proto).

The default deployment layout consists of a root `layout.cfg` file and several
subdirectories defining **source groups**, **applications**, and
**deployments**. Note that the layout depicted below is the *default* layout;
the specific config directory paths may be overridden in `layout.cfg`.

```
/layout.cfg
/sources/
  <source-group-name>/     (Defines a source group).
    <source-name>.cfg      (Defines a source within the source group).

/applications/
  <application-name>.cfg   (Defines a application).

/deployments/
  <deployment-name>.cfg    (Defines a deployment).
```

All configuration files 
* `layout.cfg`, a text protobuf file containing a `Layout` message.
* `<source-group-name>`, which defines a source group.
* `<source-name>.cfg`, which defines a `Source` message within its
  **source group** parent directory.
* `<application-name>.cfg`, which defines an `Application` message.
* `<deployment-name>.cfg`, which defines a `Deployment` message.

## Sources

**Sources** define a fully-specified repository in which **component**
configuration and data are stored. `luci_deploy` will manage the source
checkouts configured in the deployment layout.

A **source** is a file located within a source group directory named
`<source-name>.cfg`. The source is specified using the `Source` text protobuf,
defined in [config.proto](api/deploy/config.proto). Each **source** name is
unique within its **source group**, but **source** names can (and likely will)
be re-used across other **source groups** to enable deployment tracks (see
[Source Group Versioning](#source-group-versioning)).

Each **source** checkout is read-only, meaning that after the `checkout`
operation is complete, no other deployment operations will modify its contents.

### Source Initialization

**Sources** may require additional post-checkout steps to render them usable.
Such sources can offer initialization scripts that will be run by `luci_deploy`
during the `checkout` phase after the source has been checked out.

Initialization scripts are specified by adding a `SourceLayout` text protobuf
file called `luci-deploy.cfg`, to the root of the **source**. If present,
this file will be interpreted, and any initialization options present will be
included as part of the **source**'s checkout operation.

The `SourceLayout` protobuf is defined in
[checkout.proto](api/deploy/checkout.proto).

Note that the **source**'s definition *must* have `run_scripts` set to true in
order for initialization scripts to be executed. This is a security precaution
to prevent untrusted **source** repositories from adding and executing
`luci_deploy` commands without the user's permission.

### Source Group Versioning

**Applications** bind components to source *name*. Deployments then bind those
**applications** to **source groups**. Therefore, the specific **source** that
is used in a deployment is determined by the **deployment**'s
choice of **source group**.

An example is a directory layout with two source groups, one named `canary` and
one named `production`.

```
sources/canary/base.cfg
sources/production/base.cfg
```

The source named `base` is defined in both **source groups** is then used in
an **application**. At this point, the **application's** specific **source**
is not known, since the choice of *which* `base` to use requires a **source
group** to be selected. In other words, the **application** is bound to
`sources/*/base.cfg`, and the choice of which `*` to use is left to a specific
**deployment**.

A *canary* deployment can bind the **application** to the `canary` **source
group**, causing its **components** to be loaded from the version of `base`
defined in `sources/canary/base.cfg`. A *production* deployment can bind the
same **application** to the `production` **source group**, causing its
**components** to be loaded from `sources/production/base.cfg`.

### Source-Relative Paths

Paths referenced within a source are referenced using "source-relative paths".
These paths are:
* Operating system independent, as they all use "/" as their delimiter.
* Either **absolute** (starting with a "/") or **relative** (not starting with
  a "/"). Relative paths are relative to the configuration file in which they
  are specified.

## Checkout

Prior to deployment, a user must sync the checked out sources with the
configured sources using the `checkout` sub-command. This checks out *all*
sources defined in the deployment layout and runs any configured initialization
scripts to initialize them.

A `checkout` is a manual operation. If the configured **sources** change, a new
`checkout` operation must be performed to sync the on-disk checkout with the
configured sources.

Performing a checkout is simple:

```shell
$ luci_deploy checkout
```

### Local Checkout

It is oftentimes useful for a user to stage (for testing) or deploy (for triage)
code from their local checkout. Rather than editing the **source** files to use
`file:///` URLs, the user may define a set of repository overrides in their
[User Configuration](#user-configuration). Running the `checkout` sub-command
with the `--local` flag will cause the checkout to prefer the user configuration
repositroies to those configured in the **sources**.

```shell
$ luci_deploy checkout --local
```

**NOTE**: The checkout will continue to use the local overrides until the
`checkout` sub-command is re-run without the `--local` flag. However, also note
that a checkout containing a local override is considered *tainted*.

## Deployment

Deployment is accessed using the `deploy` sub-command. A user may deploy a
single component or a full deployment. All deployment operations are run in
stages, and, within each stage, in parallel.

```shell
# Deploy all Components within a Deployment.
$ luci_deploy deploy mydeployment

# Deploy a specific set of Components.
$ luci_deploy deploy mydeployment/component-a mydeployment/component-b
```

The `deploy` operation consists of several sub-stages:
1. `stage`, where deployable **components** are generated from their sources and
  configurations. See [Staging](#staging) for more information.
1. `localbuild`, where any local build operations are performed on the staged
  **components**. For some component types, this doubles as a sanity check prior
  to engaging remote services.
1. `push`, where **components** are uploaded to their remote platforms.
1. `commit`, where the uploaded **components** are activated and become live.

For testing and debugging purposes, a deployment can be stopped prior to a full
commit by using the `--stage` parameter. For example, to assert that all
**components** in a deployment can be successfully staged and locally built, a
user may run:

```shell
$ luci_deploy deploy --stage=localbuild mydeployment
```

## Component Configuration

A **component** is a single buildable/deployable unit. **Components** are
defined in **application** configuration files, and are specified as paths to
`Component` messages within a **source**. `Component` messages are defined in
[component.proto](api/deploy/component.proto).

**Components** are intentionally defined within a **source**, as opposed to the
deployment layout, because their specific composition and resource requirements
will be versioned alongside their deployable code. For example, different
versions of a Google AppEngine app may define different datastore indexes based
on the underlying functionality of their code.

## User Configuration

The user may include a configuration file in their home directory at
`~/.luci_deploy.cfg`. If present, this file will be loaded alongside the
layout configuration and incorporated into `luci_deploy` behavior.

The user configuration file is a `UserConfig` text protobuf defined in
[userconfig.proto](api/deploy/userconfig.proto).

## Staging

A staging space is created for each deployable Component. Staging offers
`luci_deploy` an isolated canvas with which it can the filesystem layouts for
that Component's actual deployment tooling to operate. All file operations,
generation, and structuring are performed in the staging state such that the
resulting staging directory is available for tooling or humans to use.

The staging space for a given Component depends on that Component's type.
A staging layout is intended to be human-navigatable while conforming to any
layout requirements imposed by that Component's deployment tooling.

One goal that is enforced is that the actual checkout directories used by
any given Component are considered read-only. This means that staging for
Components which require generated files to exist alongside source must
copy or mirror that source elsewhere. Some of the more convoluted aspects of
staging layouts are the result of this requirement.

### AppEngine

AppEngine deployments are composed of two sets of information:
* Individual AppEngine modules, including the default module.
* AppEngine Project-wide globals such as Index, Cron, Dispatch, and Queue
  settings.

The individual Components are staged and deployed independently. The globals
are composed of their respective settings in each individual Component
associated with the AppEngine project regardless of whether that Component is
actually being deployed. The aggregate globals are re-asserted once per
cloud project at the end of module deployment.

References to static content are flattened into static directories within the
staging area. The generated YAML files are configured to point to this
flattened space regardless of the original static content's location within
the source.

#### dev_appserver

Moving Component configuration into protobufs and constructing composite
GOPATH and generated configuration files prevents AppEngine tooling from
working out of the box in the source repository.

The offered solution is to manually stage the Components under test, then
run tooling against the staged Component directories. The user may optionally
install the staged paths (GOPATH, etc.) or use their default environment's
paths.

One downside to this solution is that staging operates on a snapshot of the
repository, meaning that changes to the repository won't be reflected in the
staged environment. The user can address this by either manually re-staging
the Deployment when a file changes. The user may also structure their
application such that the staged content doesn't change frequently (e.g.,
for Go, have a simple entry point that immediately imports the main app
logic). Specific structures depend on the type of Component and how it is
staged (see below).

In the future, a command to bootstrap "dev_appserver" through the staged
environment would be useful.

#### Go on Classic AppEngine

Go Classic AppEngine Components are deployed using the `appcfg.py` tool,
which is the fastest available method to deploy such applications.

Because the "app.yaml" file must exist alongside the deployed source, a
stub entry point is generated in the staging area. This stub simply imports
the actual entry point package.

The Component's Sources which declare GOPATH presence are combined in a
virutal GOPATH within the Component's staging area. This GOPATH is then
installed and `appcfg.py`'s "update" method is invoked to upload the
Component.

#### Go on Managed VM

Go AppEngine Managed VMs use a set of tools for deployment:
* `aedeploy`, an AppEngine project tool which copies the various referenced
  sources across GOPATH entries into a single GOPATH hierarchy for Docker
  isolation.
* `gcloud`, which engages the remote AppEngine service and offers deployment
  utility.
* Behind the scenes, `gcloud` uses `docker` to build the actual deployed
  image from the source (`aedeploy` target) and assembled GOPATH.

Because the "app.yaml" file must exist alongside the entry point code, the
contents of the entry package are copied (via symlink) into a generated
entry point package in the staging area.

The Component's Sources which declare GOPATH presence are combined in a
virutal GOPATH within the Component's staging area. This GOPATH is then
collapsed by `aedeploy` when the Managed VM image is built.

 NOTE: because the entry point is actually a clone of the entry point
 package, "internal/" imports will not work. This can be easily worked around
 by having the entry point import another non-internal package within the
 project, and having that package act as the actual entry point.

#### Static Content Module

The `luci_deploy` supports the concept of a static content module. This is an
AppEngine module whose sole purpose is to, via AppEngine handler definitions,
map to uploaded static content. The utility of a static module is that it can
be effortlessly updated without impacting actual AppEngine runtime processes.
Static modules can be used in conjunction with module-referencing handlers in
the default AppEngine module to create the effect of the default module
actually hosting the static content.

Static content modules are staged like other AppEngine modules, only with
no running code.

### Container Engine / Kubernetes

The `luci_deploy` supports depoying services to Google Container Engine, which
is backed by Kubernetes. A project's Container Engine configuration consists
of a series of Container Engine clusters, each of which hosts a series of
homogenous machines. Kubernetes Pods, each of which are composed of one or
more Kubernetes Components (i.e., Docker images), are deployed to one or more
Google Container Engine Clusters.

The Deployment Component defines a series of Container Engine Pods, an
amalgam of a Kubernetes Pod definition and Container Engine requirements of
that Kubernetes Pod. Each Kubernetes Pod's Component is built in its own
staging area and deployed as part of that Pod to one or more Container Engine
clusters.

The Build phase of Container Engine deployment constructs a local Docker
image of each Kubernetes Component. The Push phase pushes those images to
the remote Docker image service. The Commit phase enacts the generated
Kubernetes/ configuration which references those images on the Container
Engine configuration.

Container Engine management is done in two layers: firstly, the Container
Engine configuration is managed with `gcloud` commands. This configures:

* Which managed Clusters exist on Google Container Engine.
* What scopes, system specs, and node count each Cluster has.

Within a Cluster, several managed Kubernetes pods are deployed. The
deployment is done using the `kubectl` tool, selecting the cluster using
the "--context" flag to select the `gcloud`-generated Kubernetes context.

Each `luci_deploy` Component (Kubernetes Pod, at this level) is managed as a
Kubernetes Deployment (http://kubernetes.io/docs/user-guide/deployments/). A
`luci_deploy`-managed Deployment will have the following metadata annotations:

* "luci.managedBy", set to "luci-deploytool"
* "luci.deploytool/version" set to the `deploytool` Deployment's version
  string.
* "luci.deploytool/sourceVersion" set to the revision of the Deployment
  Component's source.

The Kubernetes Deployment for a Component will be named
"<project>--<component>".

`luci_deploy`-driven Kubernetes depoyment is fairly straightforward:
* Use "kubectl get deployments/<name>" to get the current Deployment state.
* If there is a current Deployment,
  * If its "luci.managedBy" annotation doesn't equal "luci-deploytool",
    fail. The user must manually correct this situation.
  * Check if the "luci.deploytool/version" matches the container version.
    If it does, succeed.
* Create/update the Deployment's configuration using "kubectl apply".

#### Go Container Engine Pod Components

Go Container Engine Pods use a set of tools for deployment:
* `aedeploy`, an AppEngine project tool which copies the various referenced
  sources across GOPATH entries into a single GOPATH hierarchy for Docker
  isolation.
* `gcloud`, which engages the remote AppEngine service and offers deployment
  utility.
* `docker`, which is used to build and manage the Docker images.

The Component's Sources which declare GOPATH presence are combined in a
virutal GOPATH within the Component's staging area. This GOPATH is then
collapsed by `aedeploy` when the Docker image is built.

The actual Component is built directly from the Source using `aedeploy` and
`docker build`. This is acceptable, since this is a read-only operation and
will not modify the Source.

## To Do

Following are some ideas of "planned" features that would be useful to add
to `luci_deploy`:
* Add a "manage" command to manage a specific Deployment and/or Component.
  Each invocation would load special set of subcommands based on that
  resource:
  * If the resource is a Deployment, query status?
  * Other common automatable management macros.
* Offer a `dev_appserver` fallthrough to create a staging area and
  bootstrap `dev_appserver` for the named staged AppEngine Components.
