# sk8s

This folder holds skolo related materials that are specific to kubernetes
managed resources. It may eventually be merged in ../skolo once the legacy
'push' infrastructure is turned down.

See also https://go/kubernetes-skolo-jumphosts

## metadata

This is a version of the metadata server that runs under kubernetes.

## skolo-access and skolo-revportforward

These two images handle creating tunnels that expose skolo resources, such as
the web interfaces for switches and routers, to the web, all running behind
auth-proxy. The currently configured endpoints are:

    https://rack4-router.skia.org
    https://rack4-shelf1-switch.skia.org
    https://rack4-shelf2-switch.skia.org

To do this we run the skolo-revportforward container in skolo-rack4 cluster with
one copy of the container per application we want to make available. Those
containers run revportforward and point to the device we want access to, for
example for a switch:

    revportforward ... --local_address=rack4-shelf2-poe-switch:443

Or for the router:

    revportforward ... --local_address=192.168.1.1:443

The revportforward will connect to a statefulset in skia-public. We use a
stateful set so the pod name remains fixed.

    revportforward ... --pod_name=skolo-access-0

And each different revportforward will run netcat on a different port in the
skolo-access-0 pod:

    revportforward  ... --pod_port=8000

So for the three above resources we would have:

    revportforward --kubeconfig=/etc/revportforwared/kubeconfig.yaml \
    --pod_name=skolo-access-0 \
    --pod_port=8000 \
    --local_address=192.168.1.1:443


    revportforward --kubeconfig=/etc/revportforwared/kubeconfig.yaml \
    --pod_name=skolo-access-0 \
    --pod_port=8001 \
    --local_address=rack4-shelf1-poe-switch:443


    revportforward --kubeconfig=/etc/revportforwared/kubeconfig.yaml \
    --pod_name=skolo-access-0 \
    --pod_port=8002 \
    --local_address=rack4-shelf2-poe-switch:443

The skolo-access pods contains netcat (nc), which is requires for revportforward
to work.

Finally there is the skolo-auth-proxy pod which contains multiple containers
running auth-proxy that connect to each of the ports [8000, 8001, 8002], exposed
on skolo-access-0. The auth-proxy requires login and access is restricted to the
skia-root group.
