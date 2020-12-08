import './index';

import { expect } from 'chai';
import { $, $$ } from 'common-sk/modules/dom';
import { SelectSk } from 'elements-sk/select-sk/select-sk';
import { TaskRepeaterSk } from './task-repeater-sk';
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
    ($$('select-sk', taskRepeater) as SelectSk).selection = 3;
    expect(taskRepeater).to.have.property('frequency', '7');
    (taskRepeater as TaskRepeaterSk).frequency = '2';
    expect($$('select-sk', taskRepeater)).to.have.property('selection', 2);
  });
});
