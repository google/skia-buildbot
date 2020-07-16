import { toParamSet } from 'common-sk/modules/query';
import 'elements-sk/error-toast-sk';
import fetchMock from 'fetch-mock';
import '../../../infra-sk/modules/login-sk';
import {
  ManualRollRequest,
  ModeChange,
  StrategyChange,
} from '../rpc';
import { fakeStatus } from './demo-data';

const params = toParamSet(window.location.search.substring(1));
if (params["status"]?.indexOf("error") >= 0) {
  fakeStatus["status"] = "error";
  fakeStatus["error"] = "Error message goes here!";
}

fetchMock.get("/dist/arb-status-sk.html/json/status", fakeStatus);
fetchMock.get("/loginstatus/", {
    "Email":"user@google.com",
    "LoginURL":"https://accounts.google.com/",
    "IsAGoogler":true,
});

let manualRollResult = 0;
const manualRollResults = [
  "",
  "SUCCESS",
  "FAILURE",
];
fetchMock.post("/dist/arb-status-sk.html/json/mode",
    function(url: string, opts: fetchMock.MockOptions) {
  const res = <ModeChange>JSON.parse(<string><unknown>opts.body);
  res["user"] = "you@google.com";
  if (fakeStatus.validmodes) {
    const validMode = fakeStatus.validmodes.indexOf(res.mode);
    if (validMode >= 0) {
      fakeStatus["mode"] = res;
      return fakeStatus;
    }
  }
  return new Response(
      "Invalid mode: " + res.mode + "; valid modes: " + fakeStatus.validmodes,
      {status: 400});
});
fetchMock.post("/dist/arb-status-sk.html/json/strategy",
    function(url: string, opts: fetchMock.MockOptions) {
  const res = <StrategyChange>JSON.parse(<string><unknown>opts.body);
  res["user"] = "you@google.com";
  if (!!fakeStatus.validstrategies) {
    const validStrategy = fakeStatus.validstrategies?.indexOf(res.strategy);
    if (validStrategy >= 0) {
      fakeStatus["strategy"] = res;
      return fakeStatus;
    }
  }
  return new Response(
        "Invalid strategy: " + res.strategy + "; valid strategies: " +
            fakeStatus.validstrategies,
        {status: 400});
});
fetchMock.post("/dist/arb-status-sk.html/json/unthrottle", {});
fetchMock.post("/dist/arb-status-sk.html/json/manual",
    function(url: string, opts: fetchMock.MockOptions) {
  const req = <ManualRollRequest>JSON.parse(<string><unknown>opts.body);
  req.requester = "you@google.com";
  req.result = manualRollResults[manualRollResult++ % manualRollResults.length];
  req.roller = "skia-autoroll";
  req.status = "PENDING";
  req.timestamp = new Date("2017-08-28T03:51:10Z").getTime() / 1000;
  req.url = req.result == "" ? "" : "https://fake.google.com";
  if (!fakeStatus.manualrequests) {
    fakeStatus.manualrequests = [];
  }
  fakeStatus.manualrequests.push(req);
  return req;
});

import './index.ts';