# Prober

Prober monitors the uptime of our servers and pumps the results of those probes
into Prometheus.

## Prober Files

Application specific probers should be placed in the applications top level
directory in a file called 'probersk.json'. The `build_probersk_json`
command-line application will incorporate all such files into a single prober
config file that is uploaded to the server.

## GoB expectations

Run

    make update-expectations

to update the files in `./expectations`.
