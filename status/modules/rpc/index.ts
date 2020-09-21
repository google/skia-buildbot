import { StatusService, StatusServiceClient} from './status';

export * from './status';

const host = window.location.protocol + "//" + window.location.host;
let rpcClient: StatusService = new StatusServiceClient(host, window.fetch.bind(window));

/**
 * GetAutoRollService returns an AutoRollService implementation which dispatches
 * events indicating when requests have started and ended.
 *
 * @param ele The parent element, used to dispatch events.
 */
export function GetStatusService(): StatusService {
  return rpcClient;
}

/**
 * MockRPCsForTesting switches this module to use the given AutoRollService for
 * testing purposes.
 */
export function MockRPCsForTesting(repl: StatusService) {
  rpcClient = repl;
}