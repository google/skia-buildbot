import {
  AutoRollRPCs,
  AutoRollRPCsClient,
} from "./rpc";

export * from "./rpc";

const host = window.location.protocol + "//" + window.location.host;
let rpcClient: AutoRollRPCs = new AutoRollRPCsClient(host, doFetch);

// TODO(borenet): Using "fetch" directly results in an error. I'm not sure why.
function doFetch(input: RequestInfo, init?: RequestInit | undefined): Promise<Response> {
  return fetch(input, init);
}

/**
 * GetAutoRollRPCs returns an AutoRollRPCs implementation which dispatches
 * events indicating when requests have started and ended.
 *
 * @param ele The parent element, used to dispatch events.
 */
export function GetAutoRollRPCs(ele: HTMLElement): AutoRollRPCs {
  const handler = {
    get(target: any, propKey: any, receiver: any) {
      const origMethod = target[propKey];
      return function(...args: any[]) {
        ele.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
        return origMethod.apply(rpcClient, args).then((v: any) => {
          ele.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
          return v;
        }).catch((err: any) => {
          ele.dispatchEvent(new CustomEvent('fetch-error', {
            detail: {
              error: err,
              loading: propKey,
            },
            bubbles: true,
          }));
          Promise.reject(err);
        });
      }
    }
  };
  return new Proxy(rpcClient, handler);
}

/**
 * MockRPCsForTesting switches this module to use the given AutoRollRPCs for
 * testing purposes.
 */
export function MockRPCsForTesting(repl: AutoRollRPCs) {
  rpcClient = repl;
}