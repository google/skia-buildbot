import './index';

import { setUpElementUnderTest } from '../test_util';
import { SearchControlsSk } from './search-controls-sk';

const expect = chai.expect;

describe('search-controls-sk', () => {
  const newInstance = setUpElementUnderTest('search-controls-sk');

  it('renders', () => {
    const searchControlsSk = newInstance() as SearchControlsSk;
    expect(searchControlsSk.innerText).to.equal('Hello, world!');
  });
})
