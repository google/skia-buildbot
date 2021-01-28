#!/bin/bash

# Create the named Particle scraps.
#
# The scrapexchange server should be available locally, such as forwarding the
# the production protected endpoint to localhost:
#
#    $ kubectl port-forward service/scrapexchange 9000
#
# Note that this is in place of an Admin UI for the Scrap Exchange server. Once
# that is complete it should be the canconical way to create named scraps.

# Create a name for each scrap.
curl --silent -X PUT -d "{\"Hash\": \"c5156240f082cf023c9513e878cc3c33d40c451db60a00f5adcc7c669c1300b7\", \"Description\": \"Fireworks\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@fireworks
curl --silent -X PUT -d "{\"Hash\": \"f7e261bd0844b38e1be1e3ef110dcc62d99fd840cf7e94b8cd3f29b6b4b9f14c\", \"Description\": \"Spiral\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@spiral
curl --silent -X PUT -d "{\"Hash\": \"7e28ab22af1d50676f47c4ecb17e1c944f455f43b5ccef0a82868ba42fe95550\", \"Description\": \"Double Helix\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@swirl
curl --silent -X PUT -d "{\"Hash\": \"7a637af12b5eccad978384e47bd54c222ac1a64ff0a39da76fbb1676fb6427a0\", \"Description\": \"Text\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@text
curl --silent -X PUT -d "{\"Hash\": \"e1bc35b5dc8b18aa9a93dae4195dae35229027de3d5cae844bd6cab330e65880\", \"Description\": \"Wave\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@wave
curl --silent -X PUT -d "{\"Hash\": \"93cad83247636630172e515781cf89467ca0055e8311a6048df69fe308390520\", \"Description\": \"Cube\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@cube
curl --silent -X PUT -d "{\"Hash\": \"3c2af2f8333e831edeebe0f159de40195860f296305e471eb24818ee54760972\", \"Description\": \"Confetti\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@confetti
curl --silent -X PUT -d "{\"Hash\": \"9d478ee84ed47f0f3b4fb08b50c67d4c830fa8617bb6298de92cd888f41cf350\", \"Description\": \"Uniforms Examples\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@uniforms


# List all named particle scraps.
curl --silent http://localhost:9000/_/names/particle/
