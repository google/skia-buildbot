import { assert } from 'chai';
import fetchMock from 'fetch-mock';
import { lookupCids } from './cid';
import { CommitNumber, CIDHandlerResponse } from '../json';

describe('cid/lookupCids', () => {
  it('calls the correct endpoint with the correct payload', async () => {
    const cids: CommitNumber[] = [CommitNumber(1), CommitNumber(2)];
    const mockResponse: CIDHandlerResponse = {
      commitSlice: [
        {
          offset: CommitNumber(1),
          hash: 'hash1',
          ts: 100,
          author: 'author1',
          message: 'msg1',
          url: 'url1',
          body: 'body1',
        },
        {
          offset: CommitNumber(2),
          hash: 'hash2',
          ts: 200,
          author: 'author2',
          message: 'msg2',
          url: 'url2',
          body: 'body2',
        },
      ],
      logEntry: 'log',
    };

    fetchMock.post('/_/cid/', mockResponse);

    const response = await lookupCids(cids);

    assert.deepEqual(response, mockResponse);
    assert.isTrue(fetchMock.called('/_/cid/'));
    const lastCall = fetchMock.lastCall('/_/cid/');
    assert.equal(lastCall![1]!.body, JSON.stringify(cids));

    fetchMock.restore();
  });
});
