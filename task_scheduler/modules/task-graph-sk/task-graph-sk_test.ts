import './index';
import { TaskGraphSk } from './task-graph-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';
import { job1 } from '../rpc-mock';

describe('task-graph-sk', () => {
  const newInstance = setUpElementUnderTest<TaskGraphSk>('task-graph-sk');

  let element: TaskGraphSk;
  beforeEach(() => {
    element = newInstance((el: TaskGraphSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  it('renders job1', () => {
    element.draw([job1], "fake-swarming.com");
    expect(element.getElementsByTagName("svg").length).to.equal(1);
  });
});
