import './index';
import { AutorollerStatusSk } from './autoroller-status-sk';

import { $, $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { getAutorollerStatusesResponse, SetupMocks } from '../rpc-mock';

describe('autoroller-status-sk', () => {
  const newInstance = setUpElementUnderTest<AutorollerStatusSk>('autoroller-status-sk');

  let element: AutorollerStatusSk;
  beforeEach(async () => {
    SetupMocks().expectGetAutorollerStatuses(getAutorollerStatusesResponse);
    element = newInstance();
    await new Promise((resolve) => setTimeout(resolve, 0));
  });

  describe('display', () => {
    it('statuses', () => {
      expect($('.roller', element)).to.have.length(7);
      expect($('.bg-failure', element)).to.have.length(1);
      expect($('.bg-success', element)).to.have.length(4);
      expect($('.bg-warning', element)).to.have.length(2);
    });
  });
});
