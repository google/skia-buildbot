import './index';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { $, $$ } from '../../../infra-sk/modules/dom';
import { GoldStatusSk } from './gold-status-sk';
import { StatusResponse } from '../../../golden/modules/rpc_types';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('gold-status-sk', () => {
  const newInstance = setUpElementUnderTest<GoldStatusSk>('gold-status-sk');

  let element: GoldStatusSk;
  beforeEach(async () => {
    fetchMock.getOnce('https://gold.skia.org/json/v2/trstatus', <StatusResponse>{
      corpStatus: [
        { name: 'canvaskit', untriagedCount: 0 },
        { name: 'colorImage', untriagedCount: 0 },
        { name: 'gm', untriagedCount: 13 },
        { name: 'image', untriagedCount: 0 },
        { name: 'skp', untriagedCount: 0 },
        { name: 'svg', untriagedCount: 27 },
      ],
    });
    element = newInstance();
    await fetchMock.flush(true);
  });

  describe('displays', () => {
    it('untriaged', () => {
      expect($('.tr', element)).to.have.length(6);
      expect($$('.value', element)).to.have.property('innerText', '27');
    });
  });
});
