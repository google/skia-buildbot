Compute Engine Scripts
======================

Each sub directory sets up at least one VM in GCE.
If an instance wants to use the push/pulld system for deployment it needs
to be based on the 'skia-systemd-pushable-base' snapshot that is derived from
the pushable-systemd-snapshot GCE insance.

**Important**: The 'pushable-systemd-pushable-base' is based on an Ubuntu image
and therefore automatic updating is enabled. _git_ is also installed on
skia-systemd-pushable-base by default.
VM creation scripts that use this snapshot should **not** run:

```
$ apt-get upgrade -y
```

in their startup script or during VM creation.
