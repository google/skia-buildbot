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
curl --silent -X PUT -d "{\"Hash\": \"480762f9ee5ceec1d748f190d87ea9858a8f6ed57d059f7ba13c6783eaa7a502\", \"Description\": \"Fireworks\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@fireworks
curl --silent -X PUT -d "{\"Hash\": \"b57c18b37fb4536992460f4f342966433df11dba07f6d20f1dd6cecb0609af52\", \"Description\": \"Spiral\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@spiral
curl --silent -X PUT -d "{\"Hash\": \"3a317dfa0bd381471d5878c1497a1bb55afb796f08c7e7cd5593b1dabb4fbf4f\", \"Description\": \"Double Helix\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@swirl
curl --silent -X PUT -d "{\"Hash\": \"9dfd1e4fe47ab4655bfd70e9a5361c14db9b62d12535ed675caf763653bd2e94\", \"Description\": \"Text\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@text
curl --silent -X PUT -d "{\"Hash\": \"2a3536975f5cb60daf382663c58108b6c92726b38bcc17bb45a5c7b457ccb3f0\", \"Description\": \"Wave\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@wave
curl --silent -X PUT -d "{\"Hash\": \"8a330970150e0bb63b94b8f3389b65eff009a8a85b1e55a4db83b5ea32ce673e\", \"Description\": \"Cube\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@cube
curl --silent -X PUT -d "{\"Hash\": \"1083f66b397dbfa038b1d7d1196a9496fd0526eb6dac6bc2da5d46f320a6c74a\", \"Description\": \"Confetti\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@confetti
curl --silent -X PUT -d "{\"Hash\": \"e766451e6a257788e27d8a058417da0c6c0077151461adcfcef151d34a2b5460\", \"Description\": \"Uniforms Examples\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/particle/@uniforms


# List all named particle scraps.
curl --silent http://localhost:9000/_/names/particle/
