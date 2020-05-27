import './index';

import { setUpElementUnderTest } from '../test_util';
import { ClusterPageSk } from './cluster-page-sk';

const expect = chai.expect;

describe('cluster-page-sk', () => {
  const newInstance = setUpElementUnderTest('cluster-page-sk');

  it('renders', () => {
    const clusterPageSk = newInstance() as ClusterPageSk;
    expect(clusterPageSk.innerText).to.equal('Hello, world!');
  });
})
