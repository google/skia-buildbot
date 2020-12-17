import './index';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import {
  androidRoleResp,
  generalRoleResp,
  gpuRoleResp,
  infraRoleResp,
  treeStatusResp,
} from './test_data';

fetchMock.getOnce('https://tree-status.skia.org/current', treeStatusResp);
fetchMock.getOnce('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener', generalRoleResp);
fetchMock.getOnce('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gpu-gardener', gpuRoleResp);
fetchMock.getOnce('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-android-gardener', androidRoleResp);
fetchMock.getOnce('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-infra-gardener', infraRoleResp);
Date.now = () => 1600883976659;

const el = document.createElement('tree-status-sk');
($$('#container') as HTMLElement).appendChild(el);
el.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
