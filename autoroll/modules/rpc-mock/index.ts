import { toParamSet } from 'common-sk/modules/query';
import {
  AutoRollMiniStatus,
  AutoRollMiniStatuses,
  AutoRollRPCs,
  AutoRollStatus,
  CreateManualRollRequest,
  GetRollersRequest,
  GetMiniStatusRequest,
  GetStatusRequest,
  ManualRollRequest,
  ModeChange,
  SetModeRequest,
  SetStrategyRequest,
  StrategyChange,
  UnthrottleRequest,
  UnthrottleResponse,
  MockRPCsForTesting,
} from '../rpc';

import { GetFakeStatus } from './fake-status';
import { GetFakeMiniStatuses} from './fake-ministatuses';

export * from './fake-status';

/**
 * SetupMocks changes the rpc module to use the mocked client from this module.
 */
export function SetupMocks() {
  MockRPCsForTesting(new FakeAutoRollRPCs())
}

const manualRollResults = [
  "",
  "SUCCESS",
  "FAILURE",
];

/**
 * FakeAutoRollRPCs provides a mocked implementation of AutoRollRPCs.
 */
class FakeAutoRollRPCs implements AutoRollRPCs {
  private manualRollResult: number = 0;
  private manualRequestId: number = 0;
  private status: AutoRollStatus = GetFakeStatus();

  view_GetRollers(_: GetRollersRequest): Promise<AutoRollMiniStatuses> {
    return Promise.resolve(GetFakeMiniStatuses());
  }
  view_GetMiniStatus(_: GetMiniStatusRequest): Promise<AutoRollMiniStatus> {
    return new Promise(() => this.status.ministatus);
  }
  view_GetStatus(_: GetStatusRequest): Promise<AutoRollStatus> {
    const params = toParamSet(window.location.search.substring(1));
    if (params["status"]?.indexOf("error") >= 0) {
      this.status.status = "error";
      this.status.error = "Error message goes here!";
    }
    return Promise.resolve(this.status);
  }
  edit_SetMode(req: SetModeRequest): Promise<AutoRollStatus> {
    return new Promise((resolve, reject) => {
      if (this.status.validmodes) {
        const validMode = this.status.validmodes.indexOf(req.mode);
        if (validMode < 0) {
          reject("Invalid mode: " + req.mode + "; valid modes: " + this.status.validmodes);
          return;
        }
      }
      const mc: ModeChange = {
        roller: req.roller,
        mode: req.mode,
        user: "you@google.com",
        time: new Date().getTime() / 1000,
        message: req.message,
      };
      this.status.mode = mc;
      resolve(this.status);
    });
  }
  edit_SetStrategy(req: SetStrategyRequest): Promise<AutoRollStatus> {
    return new Promise((resolve, reject) => {
      if (!!this.status.validstrategies) {
        const validStrategy = this.status.validstrategies?.indexOf(req.strategy);
        if (validStrategy < 0) {
          reject("Invalid strategy: " + req.strategy + "; valid strategies: " + this.status.validstrategies);
          return;
        }
      }
      const sc: StrategyChange = {
        roller: req.roller,
        strategy: req.strategy,
        user: "you@google.com",
        time: new Date().getTime() / 1000,
        message: req.message,
      }
      this.status.strategy = sc;
      resolve(this.status);
    });
  }
  edit_CreateManualRoll(req: CreateManualRollRequest): Promise<ManualRollRequest> {
    const result =  manualRollResults[this.manualRollResult++ % manualRollResults.length];
    const id: string = "manualRequest" + this.manualRequestId;
    this.manualRequestId++;
    const rv: ManualRollRequest = {
      id: id,
      dryrun: false,
      noemail: false,
      noresolverevision: false,
      roller: req.roller,
      revision: req.revision,
      requester: "you@google.com",
      result: result,
      status: "PENDING",
      timestamp: new Date("2017-08-28T03:51:10Z").getTime() / 1000,
      url: result == "" ? "" : "https://fake.google.com",
    };
        
    if (!this.status.manualrequests) {
      this.status.manualrequests = [];
    }
    this.status.manualrequests.push(rv);
    return Promise.resolve(rv);
  }
  edit_Unthrottle(_: UnthrottleRequest): Promise<UnthrottleResponse> {
    return Promise.resolve({});
  }
}