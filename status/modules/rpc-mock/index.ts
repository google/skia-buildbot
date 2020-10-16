/**
 * This mock client exists as an alternative to fetchMock. It mocks out the calls to the
 * Twirp-provided client directly, rather than cathing them on the network. This assumes Twirp
 * generated code works, and is to avoid implementation details of said code, such as transport
 * messages using snake_case for camelCase fields, etc.
 **/
import { MockRPCsForTesting, StatusService } from '../rpc';

import * as status from '../rpc/status';

export * from './test_data';
export * from './mock_data';

/**
 * SetupMocks changes the rpc module to use the mocked client from this module.
 */
export function SetupMocks(): MockStatusService {
  const mock = new MockStatusService();
  MockRPCsForTesting(mock);
  return mock;
}

/**
 * MockStatusService provides a mocked implementation of StatusService.
 */
export class MockStatusService implements StatusService {
  private processAddComment:
    | ((req: status.AddCommentRequest) => status.AddCommentResponse)
    | null = null;
  private processDeleteComment:
    | ((req: status.DeleteCommentRequest) => status.DeleteCommentResponse)
    | null = null;
  private processGetAutorollerStatuses:
    | ((req: status.GetAutorollerStatusesRequest) => status.GetAutorollerStatusesResponse)
    | null = null;
  private processGetIncrementalCommits:
    | ((req: status.GetIncrementalCommitsRequest) => status.GetIncrementalCommitsResponse)
    | null = null;

  constructor() {}

  exhausted(): boolean {
    return !(
      this.processAddComment ||
      this.processDeleteComment ||
      this.processGetIncrementalCommits
    );
  }

  // Set the AddComment response.
  expectAddComment(
    resp: status.AddCommentResponse,
    check: (req: status.AddCommentRequest) => void = (req) => {}
  ): MockStatusService {
    this.processAddComment = (req) => {
      check(req);
      return resp;
    };
    return this;
  }

  // Set the DeleteComment response.
  expectDeleteComment(
    resp: status.DeleteCommentResponse,
    check: (req: status.DeleteCommentRequest) => void = (req) => {}
  ): MockStatusService {
    this.processDeleteComment = (req) => {
      check(req);
      return resp;
    };
    return this;
  }

  // Set the GetIncrementalCommits response.
  expectGetIncrementalCommits(
    resp: status.GetIncrementalCommitsResponse,
    check: (req: status.GetIncrementalCommitsRequest) => void = (req) => {}
  ): MockStatusService {
    this.processGetIncrementalCommits = (req) => {
      check(req);
      return resp;
    };
    return this;
  }

  // Set the GetAutorollerStatuses response.
  expectGetAutorollerStatuses(
    resp: status.GetAutorollerStatusesResponse,
    check: (req: status.GetAutorollerStatusesRequest) => void = (req) => {}
  ): MockStatusService {
    this.processGetAutorollerStatuses = (req) => {
      check(req);
      return resp;
    };
    return this;
  }

  getIncrementalCommits(
    req: status.GetIncrementalCommitsRequest
  ): Promise<status.GetIncrementalCommitsResponse> {
    const process = this.processGetIncrementalCommits;
    this.processGetIncrementalCommits = null;
    return process ? Promise.resolve(process(req)) : Promise.reject('No mock response set');
  }

  addComment(req: status.AddCommentRequest): Promise<status.AddCommentResponse> {
    const process = this.processAddComment;
    this.processAddComment = null;
    return process ? Promise.resolve(process(req)) : Promise.reject('No mock response set');
  }

  deleteComment(req: status.DeleteCommentRequest): Promise<status.DeleteCommentResponse> {
    const process = this.processDeleteComment;
    this.processDeleteComment = null;
    return process ? Promise.resolve(process(req)) : Promise.reject('No mock response set');
  }

  getAutorollerStatuses(
    req: status.GetAutorollerStatusesRequest
  ): Promise<status.GetAutorollerStatusesResponse> {
    const process = this.processGetAutorollerStatuses;
    this.processGetAutorollerStatuses = null;
    return process ? Promise.resolve(process(req)) : Promise.reject('No mock response set');
  }
}
