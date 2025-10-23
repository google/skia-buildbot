#!/bin/bash

# Default values
INSTANCE1="tfgen-spanid-20241205020733610"
DATABASE1="chrome_int"
INSTANCE2=""
DATABASE2=""

# Parse command-line arguments
while [[ $# -gt 0 ]]; do
  key="$1"
  case $key in
    -si|--source-instance)
      INSTANCE1="$2"
      shift # past argument
      shift # past value
      ;;
    -sd|--source-database)
      DATABASE1="$2"
      shift # past argument
      shift # past value
      ;;
    -di|--destination-instance)
      INSTANCE2="$2"
      shift # past argument
      shift # past value
      ;;
    -dd|--destination-database)
      DATABASE2="$2"
      shift # past argument
      shift # past value
      ;;
    *)    # unknown option
      shift # past argument
      ;;
  esac
done

# Check if required arguments are provided
if [ -z "$INSTANCE2" ] || [ -z "$DATABASE2" ]; then
    echo "Usage: $0 -di <instance2> -dd <database2> [-si <instance1>] [-sd <database1>]"
    exit 1
fi

# Safety check to prevent using the production instance as the destination.
if [ "$INSTANCE2" == "tfgen-spanid-20241205020733610" ] || [ "$INSTANCE2" == "tfgen-spanid-20241204191122045" ]; then
    echo "Error: The destination instance cannot be the production instance."
    exit 1
fi

# Stop all containers to start afresh.
sudo docker ps -q | xargs -r sudo docker rm -vf

# port 5432 will hold connection to the source, prod db.
sudo docker run -d -p 5432:5432 -v $HOME/.config/gcloud/application_default_credentials.json:/acct_credentials.json   gcr.io/cloud-spanner-pg-adapter/pgadapter:latest   -p skia-infra-corp -i ${INSTANCE1} -d ${DATABASE1} -c /acct_credentials.json -x
# port 5433 will hold connection to the experimental instance in skia-infra-corp
sudo docker run -d -p 5433:5432 -v $HOME/.config/gcloud/application_default_credentials.json:/acct_credentials.json   gcr.io/cloud-spanner-pg-adapter/pgadapter:latest   -p skia-infra-corp -i ${INSTANCE2} -d ${DATABASE2} -c /acct_credentials.json -x
