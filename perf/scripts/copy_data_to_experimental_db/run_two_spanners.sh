# Stop all containers to start afresh.
sudo docker ps -q | xargs -r sudo docker rm -vf

# port 5432 will hold connection to the source, prod db.
sudo docker run -d -p 127.0.0.1:5432:5432 -v $HOME/.config/gcloud/application_default_credentials.json:/acct_credentials.json   gcr.io/cloud-spanner-pg-adapter/pgadapter:latest   -p skia-infra-corp -i tfgen-spanid-20241205020733610 -d chrome_int -c /acct_credentials.json -x
# port 5433 will hold connection to the experimental instance in skia-infra-corp
sudo docker run -d -p 127.0.0.1:5433:5432 -v $HOME/.config/gcloud/application_default_credentials.json:/acct_credentials.json   gcr.io/cloud-spanner-pg-adapter/pgadapter:latest   -p skia-infra-corp -i tfgen-spanid-20250415224933743 -d TEST_DB_HERE -c /acct_credentials.json -x
