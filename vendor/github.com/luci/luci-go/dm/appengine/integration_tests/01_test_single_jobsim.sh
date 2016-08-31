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

quest_id=$(add_attempts jobsim '{"return_stage": {"retval": 178}}' 1)
log $quest_id

walk_attempt "$quest_id" 1
