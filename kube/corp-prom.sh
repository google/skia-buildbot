# Sets up the port forwarding to Prometheus on skia-corp.
set -e

source ./corp-config.sh

printf "\nPress enter when finished.\n\n"
kubectl port-forward prometheus-0 9090 9090 &
pid=$!
xdg-open http://localhost:9090
read -p ""
kill $pid
