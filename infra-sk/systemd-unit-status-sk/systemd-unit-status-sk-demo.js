import './index.js'

import { $$ } from '../dom'

let report = e => { $$('#event').textContent = JSON.stringify(e.detail, null, 2); }
$$('#ele1').addEventListener('unit-action', report);
$$('#ele2').addEventListener('unit-action', report);
$$('#ele3').addEventListener('unit-action', report);

let value = {
  "status": {
    "Name": "pulld.service",
    "Description": "Skia systemd monitoring UI and pull service.",
    "LoadState": "loaded",
    "ActiveState": "active",
    "SubState": "running",
    "Followed": "",
    "Path": "/org/freedesktop/systemd1/unit/pulld_2eservice",
    "JobId": 0,
    "JobType": "",
    "JobPath": "/"
  },
  "props": {
    "AmbientCapabilities": 0,
    "AppArmorProfile": [
      false,
      ""
    ],
    "BlockIOAccounting": false,
    // ...
    "ExecMainStartTimestamp": 1516802012261906,
    // ...
    "WorkingDirectory": ""
  }
};

$$('#ele1').value = value;
value = Object.assign({}, value);
value.status.SubState = 'failed';
$$('#ele2').value = value;
value = Object.assign({}, value);
value.status.SubState = 'dead';
$$('#ele3').value = value;

