# Skolo Rack Management Design Doc

## Objective

Switch away from using the legacy 'push' system for managing jumphosts and RPi
machines and move to kubernetes.

## Overview

Right now we still need to keep our legacy push system running in order to push
applications to jumphosts, as opposed to the rest of our infrastructure which
uses kubernetes. Moving to kubernetes for the jumphosts will simplify managing
all of our infrastructure and also open the door to managing the actual test
machines via kubernetes, but that's not covered in this design doc. Note that
since each jumphost will be its own kubernetes cluster this design also
incorporates all the changes in http://go/skia-infra-multi-kubernetes.

## Detailed design

### Install

Install k3s on each jumphost.

    curl -sfL https://get.k3s.io | sh -s - -t <some secret token>

Note that k3s uses a token to bootstrap agents and needs to be supplied when
setting up the master and agents. We will use the same token for all jumphosts
so they are hot-swappable. The token will be stored in berglas secrets at
`etc/k3s-node-token`. To upgrade the kubernetes running on a jumphost just run
the command line above again.

Also note that kubernetes logs are available on the jumphost via journalctl:

    journalctl -u k3s --reverse

### attach.sh

Now that we have kubernetes running on each jumphost, how do we apply YAML files
and apply secrets? In the parlance of the old "push" mechanism, how do we push
new versions of packages?

The shell script 'infra/kube/attach.sh' leverages
'infra/kube/clusters/config.json' to set up port-forwards and sets up the
KUBECONFIG environment variable for a designated cluster and then drops the user
into a bash shell where kubectl is talking to the cluster. Since KUBECONFIG is
set per shell this will allow different shells to operation on different
clusters simultaneously.

### pushk

Pushk will be updated to be able to attach to the k3s clusters on the jumphosts,
again based on the data in `infra/kube/clusters/config.json` and the user's
`~/.ssh/id_rsa`.

### RPis

Each RPi is a node in the k8s cluster, with one cluster per rack.

See [RPI_DESIGN](./RPI_DESIGN.md) and also http://go/skolo-machine-state.

### Requirements addressed

- Apps that are running on the jumphost need access to other devices on the
  rack, for example, to back up router configs or switch ports on mPower
  devices.

  - k3s bridges the host network into the kubernetes cluster. All pods run on a
    10.0.0.0 network and k3s bridges that to the host network on 192.168.1.0,
    along with DNS. So all the devices on the rack are reachable from within
    pods running on the jumphost.

- Swarming running on devices in the rack need access to the metadata server,
  even if swarming clients are not running under kubernetes.

  - The jumphost k3s cluster will run traefik as an ingress which automatically
    exports itself on host port 80. So for metadata we just need to make sure
    DNS points 'metadata' to the jumphost (which is already done for the current
    'push' based metadata servers). Additional apps can be added by adding
    additional aliases to the /etc/hosts file on the rack router for the
    jumphost, e.g.:

          127.0.0.1   localhost.localocalhost
          192.168.1.8 jumphost metadata

## Launch plans

- Land new version of metadata that runs under kubernetes, or set up a new
  jumphost at a different IP address and then switch the DNS mapping.
- Install k3s on each jumphost and push metadata server to each.
  - Note that this is the only tricky part of the launch, as the legacy metadata
    server runs on port 80 already, so there will be a small amount of downtime
    for metadata. This will be mitigated by first deploying on skolo-internal
    which only has four bots and then deploying to the rest of the jumphosts
    once that is successful.
- The legacy metadata server will just be 'sudo systemctl stop metadata', so
  that we can quickly rollback to using it via 'sudo systemctl start metadata'.
- Push new apps
  - prometheus
  - alert-to-pubsub
- Migrate rest of legacy apps to kubernetes
  - router-backup
    - Should also be enhanced to backup switches.
  - power-cycle-daemon
  - NFS server
- Turn off legacy apps.
- Delete all 'push' code and turn down https://push.skia.org.

# Measuring improvements

When this is done we will be able to get alerts for each rack and see metrics
for each process running on the jumphosts.

# Caveats

Keeping push alive wasn't a possibility since it cuts us off from moving to
kubernetes for managing all skolo resources.

# Scalability

Since each rack has a limited physical size and k3s scales to 1000 nodes we can
safely make each rack, with a few hundred machines, a single kubernetes cluster.

# Redundancy and reliability

Eventually all the jumphosts will be identical up to, but not including, the
yaml files and secrets that are pushed to the cluster they are running. At this
point we can make a single jumphost image that allows the jumphosts to be
hot-swappable in case of failure.

# Data integrity

All the secrets are duplicated in berglas (see go/skia-infra-multi-kubernetes)
and all the yaml files are stored in git (see
https://github.com/google/skia-buildbot/blob/master/kube/README.md). Since we
will be able to run alert-to-pubsub we will gain alerting when the jumphost
processes fail.
