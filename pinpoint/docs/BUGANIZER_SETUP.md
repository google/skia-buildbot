# Buganizer Integration

This document outlines steps taken to create a new service account, grant it access to read the
IssueTracker secret key and deploy the service account with Pinpoint workers for Temporal.

0. Before performing any of the steps below, create a bug to track your work.

## Create the Service Account

When creating a service account, be concious about the naming. For example, the word "public" should
not be in the same if it is inteded to access internal information.

- Create the accounts by creating a change like
  [example](https://critique.corp.google.com/cl/617312289) and
  [example](https://skia-review.googlesource.com/c/k8s-config/+/828794).
- Grant the service account (SA) read access to CAS by creating a change like
  [example](https://critique.corp.google.com/cl/618185284).
- Grant the service account roles/cloudtrace.agent through a change like
  [example](https://critique.corp.google.com/cl/618803244).
- Create a bug like [example](https://b.corp.google.com/issues/329716920) to have the service
  account granted read access to internal repositories.
- Ensure the pinpoint SA is used by the Temporal worker by posting a change like
  [example](https://skia-review.googlesource.com/c/k8s-config/+/831596).

## Use Breakglass and grant the SA access to read the Issuetracker API Key

Please refer to
[go/skia-perf-issuetracker](http://go/skia-perf-issuetracker) for additional details.

- Grant yourself access via breakglass.

```
grants add --wait_for_twosync --reason="b/{bug_id} -- New SA" skia-infra-breakglass-policy:2h
```

- As noted in [go/skia-perf-issuetracker](http://go/skia-perf-issuetracker), grant the service
  account "Secret Manager Secret Accessor".

## Add the service account as a collaborator to newly created tickets for Chromeperf

The service account needs to be either a) a collaborator or b) have the role "issue editor" for the
component to be able to make changes to a ticket.

- Create a change like [example](https://chromium-review.googlesource.com/c/catapult/+/5405760) to
  add the service account to the list of collaborators.
- Alternatively, reach out to the Component Admin to get the service account added as an issue
  editor.
