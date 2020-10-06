import './index';
import { TaskSk } from './task-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('task-sk', () => {
  const newInstance = setUpElementUnderTest<TaskSk>('task-sk');

  let element: TaskSk;
  beforeEach(() => {
    element = newInstance((el: TaskSk) => {
      // Place here any code that must run after the element is instantiated but
      // before it is attached to the DOM (e.g. property setter calls,
      // document-level event listeners, etc.).
    });
  });

  describe('some action', () => {
    it('some result', () => {});
      expect(element).to.not.be.null;
  });
});
