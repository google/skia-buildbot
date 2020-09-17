import {
  MockRPCsForTesting, StatusService
} from '../rpc';

import { incrementalResponse0 } from './test_data';
import { GetIncrementalCommitsRequest, GetIncrementalCommitsResponse } from '../rpc/status';

export * from './test_data';
export * from './mock_data';

/**
 * SetupMocks changes the rpc module to use the mocked client from this module.
 */
export function SetupMocks(resp?: GetIncrementalCommitsResponse) {
  MockRPCsForTesting(new MockStatusService(resp || incrementalResponse0))
}

/**
 * MockStatusService provides a mocked implementation of AutoRollService.
 */
class MockStatusService implements StatusService {
  resp: GetIncrementalCommitsResponse;

  constructor(resp: GetIncrementalCommitsResponse) {
    this.resp = resp;
  }
  getIncrementalCommits(_: GetIncrementalCommitsRequest): Promise<GetIncrementalCommitsResponse> {
    return Promise.resolve(this.resp);
  }
}