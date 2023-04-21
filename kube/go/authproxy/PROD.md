# auth-proxy Production Manual

First make sure you are familiar with the design of auth-proxy by reading the
[DESIGN](http://go/auth-proxy) doc.

# Alerts

Items below here should include target links from alerts.

## refresh

Every hour the auth-proxy instance refreshes all of the allowed group members
from [CRIA](http://go/CRIA). This alert is fired if that hourly refresh has
failed at least twice in a row.

### Checklist:

- Check logs for failure details.
- Also check the service account of the pod has permissions to talk to CRIA.
- Confirm all the CRIA groups that auth-proxy is using are still defined.
- Check for a CRIA outage.
