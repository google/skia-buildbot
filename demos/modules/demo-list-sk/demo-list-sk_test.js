import './index';

import { $ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';

import { singleDemoEntry, twoDemoEntries } from './test_data';
import {
  eventPromise,
  setUpElementUnderTest,
} from '../../../golden/modules/test_util';

describe('demo-list-sk', () => {
  const newInstance = setUpElementUnderTest('demo-list-sk');
  fetchMock.config.overwriteRoutes = false;

  const loadDemolist = async (reply) => {
    const event = eventPromise('load-complete');
    fetchMock.getOnce('/demo/metadata.json', reply);
    const demolist = newInstance();
    await event;
    return demolist;
  };

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  it('shows a single entry', async () => {
    const demolist = await loadDemolist(singleDemoEntry);
    expect($('th', demolist).length).to.equal(2);
    expect($('td', demolist).length).to.equal(2);
  });

  it('shows a multiple entries', async () => {
    const demolist = await loadDemolist(twoDemoEntries);
    expect($('th', demolist).length).to.equal(2);
    expect($('td', demolist).length).to.equal(4);
  });
});
