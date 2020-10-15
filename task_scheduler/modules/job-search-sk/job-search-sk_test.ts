import './index';
import { JobSearchSk } from './job-search-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('job-search-sk', () => {
  const newInstance = setUpElementUnderTest<JobSearchSk>('job-search-sk');

  let element: JobSearchSk;
  beforeEach(() => {
    element = newInstance((el: JobSearchSk) => {
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
