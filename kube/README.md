Kubernetes config and applications
==================================

Scripts, YAML files, and utility apps to run our kubernetes cluster(s). Each
cluster will have its own subdirectory that matches the name of the GCE
project.


Admins
------

Before deploying yaml files with service accounts you need to give yourself
cluster-admin rights:

    kubectl create clusterrolebinding ${USER}-cluster-admin-binding --clusterrole=cluster-admin --user=${USER}@google.com
