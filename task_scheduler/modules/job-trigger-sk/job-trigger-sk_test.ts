import './index';
import { JobTriggerSk } from './job-trigger-sk';

import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { expect } from 'chai';

describe('job-trigger-sk', () => {
  const newInstance = setUpElementUnderTest<JobTriggerSk>('job-trigger-sk');

  let element: JobTriggerSk;
  beforeEach(() => {
    element = newInstance();
  });

  // Leave the real testing to puppeteer.
  describe('some action', () => {
    it('some result', () => {});
      expect(element).to.not.be.null;
  });
});
