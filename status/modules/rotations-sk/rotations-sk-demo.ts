import './index';
import '../tree-status-sk';
import fetchMock from 'fetch-mock';
import { $$ } from 'common-sk/modules/dom';
import {
  treeStatusResp,
  generalRoleResp,
  gpuRoleResp,
  androidRoleResp,
  infraRoleResp,
} from '../tree-status-sk/test_data';
import { TreeStatus, TreeStatusSk } from '../tree-status-sk/tree-status-sk';
import { RotationsSk } from './rotations-sk';

fetchMock.getOnce('https://tree-status.skia.org/current', treeStatusResp);
fetchMock.getOnce('https://tree-status.skia.org/current-sheriff', generalRoleResp);
fetchMock.getOnce('https://tree-status.skia.org/current-wrangler', gpuRoleResp);
fetchMock.getOnce('https://tree-status.skia.org/current-robocop', androidRoleResp);
fetchMock.getOnce('https://tree-status.skia.org/current-trooper', infraRoleResp);
Date.now = () => 1600883976659;

const ts = document.createElement('tree-status-sk') as TreeStatusSk;
const r = document.createElement('rotations-sk') as RotationsSk;
ts.addEventListener('tree-status-update', (e) => {
  r.rotations = (e as CustomEvent<TreeStatus>).detail.rotations;
});
($$('#container') as HTMLElement).appendChild(ts);
($$('#container') as HTMLElement).appendChild(r);
