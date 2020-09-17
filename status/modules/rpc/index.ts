import { StatusService, StatusServiceClient } from './status';

export * from './status';

/**
 * GetStatusService returns a StatusService implementation, either the default production one, or
 * an injected mock.
 */
export function GetStatusService(): StatusService {
  const w = window as any;
  if (!w.rpcClient) {
    const host = window.location.protocol + "//" + window.location.host;
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