#!/bin/bash

# Create the named SkSL scraps.
#
# The scrapexchange server should be available locally, such as forwarding the
# the production protected endpoint to localhost:
#
#    $ kubectl port-forward service/scrapexchange 9000
#
# Note that this is in place of an Admin UI for the Scrap Exchange server. Once
# that is complete it should be the canconical way to create named scraps.

# Create a name for each scrap.
curl --silent -X PUT -d "{\"Hash\": \"8faf0a93848175d382bc8feaef584339b75a911aa7a9d8a47834d4b05c801a08\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@inputs
curl --silent -X PUT -d "{\"Hash\": \"3124d06f75fa4a5138784e80592909d094a50dfd7a53aed82f660c1c021fa628\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@iResolution
curl --silent -X PUT -d "{\"Hash\": \"12f6c3ecf3f26b1b734d0f254b5bb97cb7d395a9e4829fbab9cda3fef9e3ad9e\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@iTime
curl --silent -X PUT -d "{\"Hash\": \"229312432fbd5c54b471d766a0da29cd7d22f53b7a9a90f3b014a87114d02dd1\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@iMouse
curl --silent -X PUT -d "{\"Hash\": \"76819961bb22c1982492ed57a3972f855a01026b0686d636bd23773f4855d218\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@default
curl --silent -X PUT -d "{\"Hash\": \"1e5f970cc8c0262496543207f2172c198617acc293e8b5ba29343f49962d4544\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@iImage
curl --silent -X PUT -d "{\"Hash\": \"1e5f970cc8c0262496543207f2172c198617acc293e8b5ba29343f49962d4544\", \"Description\": \"Shader Inputs\"}" -H 'Content-Type: application/json' http://localhost:9000/_/names/sksl/@defaultChildShader


# List all named sksl scraps.
curl --silent http://localhost:9000/_/names/sksl/
