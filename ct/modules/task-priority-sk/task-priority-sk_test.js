import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { priorities } from './test_data';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('task-priority-sk', () => {
  const factory = setUpElementUnderTest('task-priority-sk');
  // Returns a new element with the pagesets fetch complete.
  const newInstance = async () => {
    const ele = factory();
    await new Promise((resolve) => setTimeout(resolve, 0));
    return ele;
  };

  let selector; // Set at start of each test.
  beforeEach(() => {
    fetchMock.getOnce('begin:/_/task_priorities/', priorities);
  });

  afterEach(() => {
    //  Check all mock fetches called at least once and reset.
    expect(fetchMock.done()).to.be.true;
    fetchMock.reset();
  });

  it('loads selections', async () => {
    selector = await newInstance();
    expect($('select-sk div', selector)).to.have.length(3);
    expect(selector).to.have.property('priority', '100');
  });

  it('reflects changes to selection', async () => {
    selector = await newInstance();
    $$('select-sk', selector).selection = 2;
    expect(selector).to.have.property('priority', '110');
    selector.priority = '90';
    expect(selector).to.have.property('priority', '90');
    expect($$('select-sk', selector)).to.have.property('selection', 0);
  });
});
