import {
  MockRPCsForTesting, StatusService
} from '../rpc';

import { incrementalResponse0 } from './test_data';
import { GetIncrementalCommitsRequest, GetIncrementalCommitsResponse } from '../rpc/status';

export * from './test_data';

/**
 * SetupMocks changes the rpc module to use the mocked client from this module.
 */
export function SetupMocks() {
  MockRPCsForTesting(new MockStatusService())
}

/**
 * MockStatusService provides a mocked implementation of AutoRollService.
 */
class MockStatusService implements StatusService {
  getIncrementalCommits(_: GetIncrementalCommitsRequest): Promise<GetIncrementalCommitsResponse> {
    return Promise.resolve(incrementalResponse0);
  }
}