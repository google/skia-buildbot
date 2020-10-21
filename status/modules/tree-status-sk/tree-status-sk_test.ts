import './index';
import { TreeStatus, TreeStatusSk } from './tree-status-sk';

import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { $ } from 'common-sk/modules/dom';
import {
  treeStatusResp,
  generalRoleResp,
  gpuRoleResp,
  androidRoleResp,
  infraRoleResp,
} from './test_data';

describe('tree-status-sk', () => {
  const newInstance = setUpElementUnderTest<TreeStatusSk>('tree-status-sk');

  let element: TreeStatusSk;
  const createElement = async () => {
    fetchMock.getOnce('https://tree-status.skia.org/current', treeStatusResp);
    fetchMock.getOnce('https://tree-status.skia.org/current-sheriff', generalRoleResp);
    fetchMock.getOnce('https://tree-status.skia.org/current-wrangler', gpuRoleResp);
    fetchMock.getOnce('https://tree-status.skia.org/current-robocop', androidRoleResp);
    fetchMock.getOnce('https://tree-status.skia.org/current-trooper', infraRoleResp);
    Date.now = () => 1600883976659;
    element = newInstance();
    await fetchMock.flush(true);
  };

  afterEach(() => {
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  describe('displays', () => {
    it('status, user, and time', async () => {
      await createElement();
      expect($('span', element).map((e) => (e as HTMLElement).innerText)).to.deep.equal([
        'No longer Broken! ',
        '[alice 2w ago]',
      ]);
    });
  });

  describe('triggers', () => {
    it('tree-status-update event', async () => {
      const ep = eventPromise('tree-status-update');
      await createElement();
      const treeStatus = ((await ep) as CustomEvent).detail as TreeStatus;
      const rotations = treeStatus.rotations;
      expect(rotations).to.have.length(4);
      expect(rotations.map((r) => r.role)).to.deep.equal([
        'Sheriff',
        'Wrangler',
        'Robocop',
        'Trooper',
      ]);
      expect(rotations.map((r) => r.name)).to.deep.equal(['alice', 'bob', 'christy', 'dan']);
      expect(treeStatus.status).to.deep.equal(treeStatusResp);
    });
  });
});
