#/bin/bash

read -r -d '' TEMPLATE << EOM
{{range .items}}{{.metadata.name}}
  {{range .spec.containers}}{{.name}}: {{.image}}
  {{end}}
{{end}}
EOM

kubectl get pods  -o go-template --template="$TEMPLATE"

docker images gcr.io/skia-public/iap_proxy --format "{{.Repository}}:{{.Tag}}"

export NAME=skia-public/iap_proxy
export BEARER=$(curl -u _token:$(gcloud auth print-access-token) https://gcr.io/v2/token?scope=repository:$NAME:pull | cut -d'"' -f 10)
echo $BEARER
curl -H "Authorization: Bearer $BEARER" https://gcr.io/v2/$NAME/tags/list
