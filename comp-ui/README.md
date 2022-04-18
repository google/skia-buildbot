# Competitive Performance UI

Run popular browser benchmarks on a range of browsers and store the results in a
Perf instance.

[Design Doc](http://go/comp-ui)

## Deployment

The comp-ui-cron-job needs to be built and deployed via an Ansible script since
it runs in the skolo and contains an embedded service account key that allows it
to upload the run results to the Google Cloud Storage bucket.

    cd skolo/ansible
    ansible-playbook ./switchboard/build_and_release_compui.yml

Wait for the created CL to land, then run:

    ansible-playbook ./switchboard/install_compui.yml
