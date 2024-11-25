import './index';

import sinon from 'sinon';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { $ } from '../../../infra-sk/modules/dom';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { TreeStatus, TreeStatusSk } from './tree-status-sk';
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
    sinon.stub(Notification, 'permission').value('denied');
    fetchMock.get('https://test-tree-status/test-repo/current', treeStatusResp);
    fetchMock.get(
      'https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gardener',
      generalRoleResp
    );
    fetchMock.get(
      'https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-gpu-gardener',
      gpuRoleResp
    );
    fetchMock.get(
      'https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-android-gardener',
      androidRoleResp
    );
    fetchMock.get(
      'https://chrome-ops-rotation-proxy.appspot.com/current/grotation:skia-infra-gardener',
      infraRoleResp
    );
    Date.now = () => 1600883976659;
    element = newInstance((el) => {
      el.baseURL = 'https://test-tree-status';
      el.repo = 'test-repo';
    });
    await fetchMock.flush(true);
  };

  afterEach(() => {
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
    sinon.restore();
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
      expect(rotations.map((r) => r.role)).to.deep.equal(['Skia', 'GPU', 'Android', 'Infra']);
      expect(rotations.map((r) => r.name)).to.deep.equal(['alice', 'bob', 'christy', 'dan']);
      expect(treeStatus.status).to.deep.equal(treeStatusResp);
    });
  });
});
