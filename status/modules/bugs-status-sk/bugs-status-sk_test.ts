import './index';
import { expect } from 'chai';
import fetchMock from 'fetch-mock';
import { $, $$ } from 'common-sk/modules/dom';
import { BugsStatusSk } from './bugs-status-sk';
import { GetClientCountsResponse, StatusData } from '../../../bugs-central/modules/json';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('bugs-status-sk', () => {
  const newInstance = setUpElementUnderTest<BugsStatusSk>('bugs-status-sk');

  let element: BugsStatusSk;
  beforeEach(async () => {
    fetchMock.getOnce('https://bugs-central.skia.org/get_client_counts', <GetClientCountsResponse>{
      clients_to_status_data: {
        Android: <StatusData>{
          untriaged_count: 10,
          link: 'www.test-link.com/test1',
        },
        Chromium: <StatusData>{
          untriaged_count: 23,
          link: 'www.test-link.com/test2',
        },
        Skia: <StatusData>{
          untriaged_count: 104,
          link: 'www.test-link.com/test3',
        },
      },
    });
    element = newInstance();
    await fetchMock.flush(true);
  });

  afterEach(() => {
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  describe('displays', () => {
    it('untriaged', () => {
      expect($('.tr', element)).to.have.length(3);
      expect($$('.value', element)).to.have.property('innerText', '10');
    });
  });
});
