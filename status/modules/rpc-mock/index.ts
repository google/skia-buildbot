/**
 * This mock client exists as an alternative to fetchMock. It mocks out the calls to the
 * Twirp-provided client directly, rather than cathing them on the network. This assumes Twirp
 * generated code works, and is to avoid implementation details of said code, such as transport
 * messages using snake_case for camelCase fields, etc.
 **/
import { MockRPCsForTesting, StatusService } from '../rpc';

import { incrementalResponse0 } from './test_data';
import * as status from '../rpc/status';

export * from './test_data';
export * from './mock_data';

/**
 * SetupMocks changes the rpc module to use the mocked client from this module.
 */
export function SetupMocks(resp?: status.GetIncrementalCommitsResponse) {
  MockRPCsForTesting(new MockStatusService(resp || incrementalResponse0));
}

/**
 * MockStatusService provides a mocked implementation of StatusService.
 */
class MockStatusService implements StatusService {
  resp: status.GetIncrementalCommitsResponse;

  constructor(resp: status.GetIncrementalCommitsResponse) {
    this.resp = resp;
  }
  getIncrementalCommits(
    _: status.GetIncrementalCommitsRequest
  ): Promise<status.GetIncrementalCommitsResponse> {
    return Promise.resolve(this.resp);
  }
  addComment(_: status.AddCommentRequest): Promise<status.AddCommentResponse> {
    return Promise.resolve({});
  }
  deleteComment(_: status.DeleteCommentRequest): Promise<status.DeleteCommentRequest> {
    return Promise.resolve({});
  }
}
