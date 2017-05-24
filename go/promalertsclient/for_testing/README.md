To test the promalerts client locally, first set up an SSH tunnel from localhost:8001 to prom-skia:8001:

     gcloud compute ssh --zone us-central1-c --project google.com:skia-buildbots skia-prom -- -v -L 8001:skia-prom:8001
     go run ./main.go --alerts_endpoint localhost:8001

