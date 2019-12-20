PODNAME=$(kubectl get pods -lname=rpi-swarming -o jsonpath='{.items[0].metadata.name}')
GOOS=linux GOARCH=arm GOARM=7 go build -o out/bot_config -v ./go/bot_config
echo ${PODNAME}
kubectl cp out/bot_config ${PODNAME}:/tmp/bot_config -c rpi-swarming-client
echo "{\"foo\": \"bar\"}" | kubectl exec -i ${PODNAME} -c rpi-swarming-client -- /tmp/bot_config get_state