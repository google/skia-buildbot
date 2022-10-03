import './index';
import { $, $$ } from 'common-sk/modules/dom';
import { AlertConfigSk } from './alert-config-sk';
import { Alert } from '../json';

window.perf = window.perf || {};
window.perf.key_order = [];
window.perf.display_group_by = true;

// Force all the alert-config-sk controls on the page to re-render.
const refreshControls = () => {
  $<AlertConfigSk>('alert-config-sk')!.forEach((ele) => {
    // eslint-disable-next-line no-self-assign
    ele.paramset = ele.paramset;
  });
};

$$('#display_group_by')!.addEventListener('click', () => {
  window.perf.display_group_by = true;
  refreshControls();
});
$$('#hide_group_by')!.addEventListener('click', () => {
  window.perf.display_group_by = false;
  refreshControls();
});

const paramset = {
  config: ['565', '8888'],
  type: ['CPU', 'GPU'],
  units: ['ms', 'bytes'],
  test: [
    'DeferredSurfaceCopy_discardable',
    'DeferredSurfaceCopy_nonDiscardable',
    'GLInstancedArraysBench_instance',
    'GLInstancedArraysBench_one_0',
    'GLInstancedArraysBench_one_1',
    'GLInstancedArraysBench_one_2',
    'GLInstancedArraysBench_one_4',
    'GLInstancedArraysBench_one_8',
    'GLInstancedArraysBench_two_0',
    'GLInstancedArraysBench_two_1',
    'GLInstancedArraysBench_two_2',
    'GLInstancedArraysBench_two_4',
    'GLInstancedArraysBench_two_8',
    'GLVec4ScalarBench_scalar_1_stage',
    'GLVec4ScalarBench_scalar_2_stage',
  ],
};

const config: Alert = {
  id_as_string: '1',
  sparse: false,
  step_up_only: false,
  display_name: 'A name',
  direction: 'BOTH',
  query: 'config=565',
  alert: 'alerts@example.com',
  interesting: 25,
  step: 'cohen',
  bug_uri_template: 'http://example.com/{description}/{url}',
  algo: 'stepfit',
  owner: 'somebody@example.org',
  minimum_num: 2,
  category: 'experimental',
  state: 'DELETED',
  group_by: 'config,units',
  radius: 7,
  k: 50,
};

const keyOrder = ['test', 'units'];

document
  .querySelectorAll<AlertConfigSk>('alert-config-sk')
  .forEach((element) => {
    element.paramset = paramset;
    element.config = config;
    element.key_order = keyOrder;
  });

const state = document.querySelector('#state')!;

const ele = document.querySelector<AlertConfigSk>('alert-config-sk')!;
window.setInterval(() => {
  state.textContent = JSON.stringify(ele.config, null, '  ');
}, 100);
