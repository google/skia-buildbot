#/bin/bash

kubectl port-forward service/prometheus 8000:8000 &
PID=$!
sleep 2
curl -X POST http://localhost:8000/-/reload
kill ${PID}
