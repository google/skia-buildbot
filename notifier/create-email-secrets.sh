# Create the secrets needed for notifier to send email.
set -e

# Enable the gmail API for your project and create a client secret for this applicaiton.
# Then download the client_secret.json file.
echo "Download client_secret.json for notifier to the email_secrets directory."
read -r -p "Press enter to continue..." key

# Then run 'three_legged_flow' in this directory and when prompted authorize as alertserver@skia.org to create the client_token.json file.
go install ./go/three_legged_flow
cd email_secrets
three_legged_flow --scopes=https://mail.google.com/
cd ..
kubectl create secret generic notifier-email-secrets --from-file=email_secrets=.. --dry-run -o yaml | kubectl apply -f -

# Finally, remove the token file since it contains a refresh token.
rm client_token.json
