# Role Name

`install_prometheus_and_alert_to_pubsub`

## Description

Builds alert-to-pubsub and deploys it along with Prometheus to each of the
jumphosts.

## Requirements

The default service account key for `chrome-bot` must have permissions to push
PubSub messages to the alert topic.

## Example Playbook

    - hosts: jumphosts

      roles:
        - install_prometheus_and_alert_to_pubsub
