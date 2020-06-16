import './index';

import { $, $$ } from 'common-sk/modules/dom';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('task-repeater-sk', () => {
  const newInstance = setUpElementUnderTest('task-repeater-sk');

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
