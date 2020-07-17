import {
  AutoRollRPCs,
  AutoRollRPCsClient,
} from "./rpc";

export * from "./rpc";

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
  const host = window.location.protocol + "//" + window.location.host;
  const client = new AutoRollRPCsClient(host, doFetch);
  const handler = {
    get(target: any, propKey: any, receiver: any) {
      const origMethod = target[propKey];
      return function(...args: any[]) {
        ele.dispatchEvent(new CustomEvent('begin-task', { bubbles: true }));
        return origMethod.apply(client, args).then((v: any) => {
          ele.dispatchEvent(new CustomEvent('end-task', { bubbles: true }));
          return v;
        }, (err: any) => {
          ele.dispatchEvent(new CustomEvent('fetch-error', {
          detail: {
            error: err,
            loading: propKey,
          },
          bubbles: true,
          }));
          return err;
        }).catch((err: any) => {
          ele.dispatchEvent(new CustomEvent('fetch-error', {
          detail: {
            error: err,
            loading: propKey,
          },
          bubbles: true,
          }));
          throw err;
        });
      }
    }
  };
  return new Proxy(client, handler);
}