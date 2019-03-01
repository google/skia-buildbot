
Enable k8s to pull images from Container Registry
=================================================

- Create a service account, i.e. skolo-gcr-pull
- Download the key, e.g. svacc.json
- Create a secret within Kubernetes so it can download images:
```
  kubectl create secret docker-registry skolo-gcr-json-key \
   --docker-server=https://gcr.io \
   --docker-username=_json_key \
   --docker-password="$(cat ~/path/to/svc-account.json)" \
   --docker-email=skolo-gcr-pull@skia-public.iam.gserviceaccount.com

  kubectl patch serviceaccount default -p '{"imagePullSecrets": [{"name": "skolo-gcr-json-key"}]}'

  kubectl get serviceaccount default -o yaml
```
// see: https://ryaneschinger.com/blog/using-google-container-registry-gcr-with-minikube/


