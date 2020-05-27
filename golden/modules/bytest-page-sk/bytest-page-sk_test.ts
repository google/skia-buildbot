import './index';

import { setUpElementUnderTest } from '../test_util';
import { BytestPageSk } from './bytest-page-sk';

const expect = chai.expect;

describe('bytest-page-sk', () => {
  const newInstance = setUpElementUnderTest('bytest-page-sk');

  it('renders', () => {
    const bytestPageSk = newInstance() as BytestPageSk;
    expect(bytestPageSk.innerText).to.equal('Hello, world!');
  });
})
