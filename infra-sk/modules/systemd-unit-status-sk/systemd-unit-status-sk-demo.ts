import './index';

import { $$ } from 'common-sk/modules/dom';
import { SystemdUnitStatusSk, SystemdUnitStatusSkEventDetail } from './systemd-unit-status-sk';
import { SystemdUnitStatus } from './json';

const report = (e: CustomEvent<SystemdUnitStatusSkEventDetail>) => {
  $$('#event')!.textContent = JSON.stringify(e.detail, null, 2);
};

$$('#ele1')!.addEventListener(
  'unit-action',
  (e) => report(e as CustomEvent<SystemdUnitStatusSkEventDetail>),
);
$$('#ele2')!.addEventListener(
  'unit-action',
  (e) => report(e as CustomEvent<SystemdUnitStatusSkEventDetail>),
);
$$('#ele3')!.addEventListener(
  'unit-action',
  (e) => report(e as CustomEvent<SystemdUnitStatusSkEventDetail>),
);

let value: SystemdUnitStatus = {
  status: {
    Name: 'pulld.service',
    Description: 'Skia systemd monitoring UI and pull service.',
    LoadState: 'loaded',
    ActiveState: 'active',
    SubState: 'running',
    Followed: '',
    Path: '/org/freedesktop/systemd1/unit/pulld_2eservice',
    JobId: 0,
    JobType: '',
    JobPath: '/',
  },
  props: {
    AmbientCapabilities: 0,
    AppArmorProfile: [
      false,
      '',
    ],
    BlockIOAccounting: false,
    // ...
    ExecMainStartTimestamp: 1516802012261906,
    // ...
    WorkingDirectory: '',
  },
};

$$<SystemdUnitStatusSk>('#ele1')!.value = value;
value = Object.assign({}, value);
value.status!.SubState = 'failed';
$$<SystemdUnitStatusSk>('#ele2')!.value = value;
value = Object.assign({}, value);
value.status!.SubState = 'dead';
$$<SystemdUnitStatusSk>('#ele3')!.value = value;
