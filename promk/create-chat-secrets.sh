# Create the secrets needed for alertmanager_webhooks to send email.
set -e

# Enable the gmail API for your project and create a client secret for this applicaiton.
# Then download the client_secret.json file.
echo "Download client_secret.json for alertmanager_webhooks."
read -r -p "Press enter to continue..." key

# Then run 'three_legged_flow' in this directory and when prompted authorize as alertserver@skia.org to create the client_token.json file.
go install ./go/three_legged_flow
three_legged_flow --scopes=https://www.googleapis.com/auth/gmail.send
kubectl create secret generic alertmanager-webhook-client-secret --from-file=client_secret.json=client_secret.json --dry-run -o yaml | kubectl apply -f -
kubectl create secret generic alertmanager-webhook-client-token --from-file=client_token.json=client_token.json --dry-run -o yaml | kubectl apply -f -

# Finally, remove the token file since it contains a refresh token.
rm client_token.json
