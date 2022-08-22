import { toParamSet } from 'common-sk/modules/query';
import {
  AutoRollService,
  AutoRollStatus,
  CreateManualRollRequest,
  GetRollersRequest,
  GetMiniStatusRequest,
  GetStatusRequest,
  ManualRoll,
  ModeChange,
  SetModeRequest,
  SetStrategyRequest,
  StrategyChange,
  UnthrottleRequest,
  UnthrottleResponse,
  MockRPCsForTesting,
} from '../rpc';

import { GetFakeStatus } from './fake-status';
import { GetFakeMiniStatuses } from './fake-ministatuses';
import {
  GetRollersResponse, Mode, Strategy, ManualRoll_Status, GetStatusResponse, GetMiniStatusResponse, SetModeResponse, SetStrategyResponse, CreateManualRollResponse, ManualRoll_Result,
} from '../rpc/rpc';
import { GetFakeConfig } from './fake-config';

export * from './fake-status';

/**
 * SetupMocks changes the rpc module to use the mocked client from this module.
 */
export function SetupMocks() {
  MockRPCsForTesting(new FakeAutoRollService());
}

const manualRollResults = Object.keys(ManualRoll_Result);

/**
 * FakeAutoRollService provides a mocked implementation of AutoRollService.
 */
class FakeAutoRollService implements AutoRollService {
  private manualRollResult: number = 0;

  private manualRequestId: number = 0;

  private status: AutoRollStatus = GetFakeStatus();

  getRollers(_: GetRollersRequest): Promise<GetRollersResponse> {
    return Promise.resolve({
      rollers: GetFakeMiniStatuses(),
    });
  }

  getMiniStatus(_: GetMiniStatusRequest): Promise<GetMiniStatusResponse> {
    return Promise.resolve({
      status: this.status.miniStatus,
    });
  }

  getStatus(_: GetStatusRequest): Promise<GetStatusResponse> {
    const params = toParamSet(window.location.search.substring(1));
    if (params.status?.indexOf('error') >= 0) {
      this.status.status = 'error';
      this.status.error = 'Error message goes here!';
    }
    return Promise.resolve({
      status: this.status,
    });
  }

  setMode(req: SetModeRequest): Promise<SetModeResponse> {
    return new Promise((resolve, reject) => {
      const validModes = Object.keys(Mode);
      const validMode = validModes.indexOf(req.mode);
      if (validMode < 0) {
        reject(`Invalid mode: ${req.mode}; valid modes: ${validModes}`);
        return;
      }
      const mc: ModeChange = {
        rollerId: req.rollerId,
        mode: req.mode,
        user: 'you@google.com',
        time: new Date().toString(), // TODO(borenet): Is this the right format?
        message: req.message,
      };
      this.status.mode = mc;
      resolve({
        status: this.status,
      });
    });
  }

  setStrategy(req: SetStrategyRequest): Promise<SetStrategyResponse> {
    return new Promise((resolve, reject) => {
      const validStrategies = Object.keys(Strategy);
      const validStrategy = validStrategies.indexOf(req.strategy);
      if (validStrategy < 0) {
        reject(`Invalid strategy: ${req.strategy}; valid strategies: ${
          validStrategies}`);
        return;
      }
      const sc: StrategyChange = {
        rollerId: req.rollerId,
        strategy: req.strategy,
        user: 'you@google.com',
        time: new Date().toString(), // TODO(borenet): Is this the right format?
        message: req.message,
      };
      this.status.strategy = sc;
      resolve({
        status: this.status,
      });
    });
  }

  createManualRoll(req: CreateManualRollRequest): Promise<CreateManualRollResponse> {
    const result = manualRollResults[this.manualRollResult++ % manualRollResults.length];
    const id: string = `manualRequest${this.manualRequestId}`;
    this.manualRequestId++;
    const rv: ManualRoll = {
      id: id,
      dryRun: false,
      canary: false,
      noEmail: false,
      noResolveRevision: false,
      rollerId: req.rollerId,
      revision: req.revision,
      requester: 'you@google.com',
      result: ManualRoll_Result[<keyof typeof ManualRoll_Result>result],
      status: ManualRoll_Status.PENDING,
      timestamp: new Date('2017-08-28T03:51:10Z').toString(), // TODO(borenet): Is this the right format?
      url: result == '' ? '' : 'https://fake.google.com',
    };
    if (!this.status.manualRolls) {
      this.status.manualRolls = [];
    }
    this.status.manualRolls.push(rv);
    return Promise.resolve({
      roll: rv,
    });
  }

  unthrottle(_: UnthrottleRequest): Promise<UnthrottleResponse> {
    return Promise.resolve({});
  }
}
