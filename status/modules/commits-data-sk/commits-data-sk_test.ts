import './index';
import { $, $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('commits-data-sk', () => {
  /**
   * 
   * TODO make tests for this element. the test_data we have in this branch is static,
   * but the commits-table-sk imrpoves it to being dynamic, but thats bad for testing,
   * so use this as is, then take the test_data in the newer branch and make it 'mock' or 'demo' data.
   * 
   * 
   */
  const newInstance = setUpElementUnderTest('commits-data-sk');

  it('displays options', () => {
    const taskRepeater = newInstance();
    expect($('select-sk div', taskRepeater)).to.have.length(4);
    expect(taskRepeater).to.have.property('frequency', '0');
  });

  it('reflects changes in selection', () => {
    const taskRepeater = newInstance();
    $$('select-sk', taskRepeater).selection = 3;
    expect(taskRepeater).to.have.property('frequency', '7');
    taskRepeater.frequency = '2';
    expect($$('select-sk', taskRepeater)).to.have.property('selection', 2);
  });
});