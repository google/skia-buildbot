import './index';
import { $$ } from 'common-sk/modules/dom';
import fetchMock from 'fetch-mock';
import {
  androidRoleResp,
  generalRoleResp,
  gpuRoleResp,
  infraRoleResp,
  treeStatusResp, treeStatusResp2, treeStatusResp3,
} from './test_data';

fetchMock.get('https://tree-status.skia.org/current', () => getTreeStatusResp());
fetchMock.get('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener', generalRoleResp);
fetchMock.get('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gpu-gardener', gpuRoleResp);
fetchMock.get('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-android-gardener', androidRoleResp);
fetchMock.get('https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-infra-gardener', infraRoleResp);
Date.now = () => 1600883976659;

let treeStatusCalledNum = 0;
function getTreeStatusResp(): fetchMock.MockResponse {
  treeStatusCalledNum++;
  if (treeStatusCalledNum === 1) {
    return treeStatusResp;
  }
  if (treeStatusCalledNum === 2) {
    return treeStatusResp2;
  }
  return treeStatusResp3;
}

const el = document.createElement('tree-status-sk');
($$('#container') as HTMLElement).appendChild(el);
el.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
