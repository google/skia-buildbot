import './index';
import { $$ } from 'common-sk/modules/dom';
import {
  androidRoleResp,
  generalRoleResp,
  gpuRoleResp,
  infraRoleResp,
  treeStatusResp,
} from './test_data';
import fetchMock from 'fetch-mock';

fetchMock.getOnce('https://tree-status.skia.org/current', treeStatusResp);
fetchMock.getOnce('https://tree-status.skia.org/current-sheriff', generalRoleResp);
fetchMock.getOnce('https://tree-status.skia.org/current-wrangler', gpuRoleResp);
fetchMock.getOnce('https://tree-status.skia.org/current-robocop', androidRoleResp);
fetchMock.getOnce('https://tree-status.skia.org/current-trooper', infraRoleResp);
Date.now = () => 1600883976659;

const el = document.createElement('tree-status-sk');
($$('#container') as HTMLElement).appendChild(el);
el.addEventListener('some-event-name', (e) => {
  document.querySelector('#events')!.textContent = JSON.stringify(e, null, '  ');
});
