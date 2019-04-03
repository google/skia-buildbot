#/bin/bash

# Creates the SSL cert used by k8s in skia-corp.

set -e -x
source ../kube/corp-config.sh
source ../bash/ramdisk.sh


SUBJ="
C=US
ST=New York
O=
localityName=New York City
commonName=*
organizationalUnitName=Skia
emailAddress=skiabot@google.com
"

cd /tmp/ramdisk

# Create the SSL cert. Details here:
# https://kubernetes.io/docs/user-guide/ingress/#tls
openssl genrsa -out tls.key 2048
openssl req -new -subj "$(echo -n "$SUBJ" | tr "\n" "/")" -key tls.key -out tls.csr -passin pass:
openssl x509 -req -days 365 -in tls.csr -signkey tls.key -out tls.crt

# Create the k8s secret.
cat > secret.yaml <<- EOM
apiVersion: v1
data:
  tls.crt: $(cat tls.crt | base64 -w 0)
  tls.key: $(cat tls.key | base64 -w 0)
kind: Secret
metadata:
  name: skia-corp-tls
  namespace: default
type: kubernetes.io/tls
EOM

kubectl apply -f secret.yaml

cd -
