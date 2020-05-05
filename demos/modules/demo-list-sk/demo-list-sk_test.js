/* eslint-disable no-unused-expressions */
/* eslint-disable no-undef */
import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';

import { singleDemoEntry, twoDemoEntries
} from './test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../golden/modules/test_util';

describe('demo-list-sk', () => {
  const newInstance = setUpElementUnderTest('demo-list-sk');
  fetchMock.config.overwriteRoutes = false;

  const loadDemolist = (reply) => {
    fetchMock.getOnce('/demo/metadata.json', reply);
    const demolist = newInstance();
    return demolist;
  };

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  it('shows a single entry', () => {
    const demolist = loadDemolist(singleDemoEntry);
    //console.log(demolist);
    // (3 items) * 6 columns
    expect($('td', demolist).length).to.equal(18);
  });

});
