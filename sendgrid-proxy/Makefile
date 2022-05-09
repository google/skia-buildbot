build:
	bazelisk build //sendgrid-proxy/...

# The keys from SendGrid are managed at
# https://app.sendgrid.com/settings/api_keys.
#
# The sendgrid-proxy application only loads the API Key on startup, so if the
# API Key gets updated make sure to restart all sendgrid-proxy pods.
update-sendgrid-api-key-secret:
	bazelisk run //kube/go/secrets:secrets -- update skia-public sendgrid-proxy
