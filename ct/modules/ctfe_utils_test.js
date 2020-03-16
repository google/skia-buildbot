import { getTimestamp } from './ctfe_utils';

describe('ctfe_utils', () => {

  afterEach(() => {
  });

  it('getTimestamp works', async () => {
    let date = getTimestamp(20200222200202);
    expect(date).to.equal({});
  });
});
