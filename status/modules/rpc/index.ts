import { StatusService, StatusServiceClient } from './status';

export * from './status';

/**
 * GetStatusService returns a StatusService implementation, either the default production one, or
 * an injected mock. We use the mock client rather than fetchMock to avoid testing around
 * implementation details of the Twirp generated client/server protocol.
 */
export function GetStatusService(): StatusService {
  const w = window as any;
  // We use a lazy window property to keep our client, so it is truly global (to support client
  // injection), and initialized exactly once, rather than per import of this module.
  if (!w.rpcClient) {
    const host = window.location.protocol + '//' + window.location.host;
    w.rpcClient = new StatusServiceClient(host, window.fetch.bind(window));
  }
  return w.rpcClient;
}

/**
 * MockRPCsForTesting switches this module to use the given StatusService for
 * testing purposes.
 */
export function MockRPCsForTesting(repl: StatusService) {
  (<any>window).rpcClient = repl;
}
