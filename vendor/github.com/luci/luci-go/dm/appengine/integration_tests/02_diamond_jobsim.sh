#!/usr/bin/env bash

# Copyright 2016 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

. ./integration_test.sh

write_config services/luci-dm/distributors.cfg <<EOF
distributor_configs: {
  key: "jobsim"
  value: { jobsim: {} }
}
EOF

read -r -d '' job_text <<EOJ
{
  "name": "hello",
  "stages": [
    {
      "deps": {"deps": [{
        "shards": 6,
        "attempts": {"items": [{"single": 1}]},
        "mix_seed": true,
        "phrase": {
          "name": "compile",
          "stages": [
            {
              "deps": {"deps": [{
                "shards": 6,
                "attempts": {"items": [{"single": 1}]},
                "phrase": {
                  "name": "thingy",
                  "stages": [
                    {
                      "deps": {"deps": [{
                        "attempts": {"items": [{"single": 1}]},
                        "phrase": {
                          "name": "nerp",
                          "stages": [
                            {
                              "deps": {"deps": [{
                                "attempts": {"items": [{"single": 1}]},
                                "phrase": {
                                  "seed": 1,
                                  "name": "isolate_source",
                                  "return_stage": {"retval": 1}
                                }
                              }]}
                            }
                          ],
                          "return_stage": {"retval": 1}
                        }
                      }]}
                    }
                  ],
                  "return_stage": {"retval": 1}
                }
              }]}
            }
          ],
          "return_stage": {"retval": 1}
        }
      }]}
    }
  ]
}
EOJ

quest_id=$(add_attempts jobsim "$job_text" 1)
log $quest_id

walk_attempt "$quest_id" 1
