import './index';
import { JobTimelineSk } from './job-timeline-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('job-timeline-sk', () => {
  const newInstance = setUpElementUnderTest<JobTimelineSk>('job-timeline-sk');

  let element: JobTimelineSk;
  beforeEach(() => {
    element = newInstance((el: JobTimelineSk) => {
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
