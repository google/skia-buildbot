import { expect } from 'chai';
import { TaskGraphSk } from './task-graph-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { job1 } from '../rpc-mock';


describe('task-graph-sk', () => {
  const newInstance = setUpElementUnderTest<TaskGraphSk>('task-graph-sk');

  let element: TaskGraphSk;
  beforeEach(() => {
    element = newInstance();
  });

  it('renders job1', () => {
    element.draw([job1], "fake-swarming.com");
    expect(element.childNodes.length).to.equal(1);
    expect(element.childNodes[0].nodeName).to.equal("svg");
  });
});
