import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { buildsJson } from './test_data';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('chromium-build-selector-sk', () => {
  const factory = setUpElementUnderTest('chromium-build-selector-sk');
  // Returns a new element with the pagesets fetch complete.
  const newInstance = async (init) => {
    const ele = factory(init);
    await fetchMock.flush(true);
    return ele;
  };

  let selector; // Set at start of each test.
  beforeEach(() => {
    fetchMock.postOnce('begin:/_/get_chromium_build_tasks', buildsJson);
  });

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  it('loads selections', async () => {
    selector = await newInstance();
    expect($('select-sk div')).to.have.length(1);
    expect(selector).to.have.deep.property('build', buildsJson.data[0]);
  });
});
