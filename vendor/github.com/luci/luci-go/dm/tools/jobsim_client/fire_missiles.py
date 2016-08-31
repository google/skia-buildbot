#!/usr/bin/env python
# Copyright 2016 The LUCI Authors. All rights reserved.
# Use of this source code is governed under the Apache License, Version 2.0
# that can be found in the LICENSE file.

import json

def dumps(obj):
  return json.dumps(obj, sort_keys=True, separators=(',', ':'))

distParams = {
  "scheduling": {
    "dimensions": {
      "cpu": "x86-64",
      "os": "Linux",
      "pool": "default",
    },
  },
  "meta": {"name_suffix": "dm test"},
  "job": {
    "inputs": {
      "isolated": [
        {"id": "01e25aad5365fc54a4b05971c15b84933bbe891a"}
      ]
    },
    "command": ["jobsim_client", "edit-distance", "-dm-host", "${DM.HOST}",
                "-execution-auth-path", "${DM.EXECUTION.AUTH:PATH}",
                "-quest-desc-path", "${DM.QUEST.DATA.DESC:PATH}"],
  }
}

params = {
  "a": "ca",
  "b": "ha",
}

desc = {
  "quest": [
    {
      "distributor_config_name": "swarming",
      "parameters": dumps(params),
      "distributor_parameters": dumps(distParams),
      "meta": {
        "timeouts": {
          "start": "600s",
          "run": "300s",
          "stop": "300s",
        }
      },
    }
  ],
  "quest_attempt": [
    {"nums": [1]},
  ]
}

print dumps(desc)
