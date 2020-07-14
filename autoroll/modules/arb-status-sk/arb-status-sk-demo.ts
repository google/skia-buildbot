import { toParamSet } from 'common-sk/modules/query';
import 'elements-sk/error-toast-sk';
import fetchMock from 'fetch-mock';
import '../../../infra-sk/modules/login-sk';
import { ManualRollRequest, Mode, Status, Strategy } from './arb-status-sk';
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
  const res = <Mode>JSON.parse(<string><unknown>opts.body);
  res["user"] = "you@google.com";
  const validMode = fakeStatus["validModes"].indexOf(res.mode);
  if (validMode >= 0) {
    fakeStatus["mode"] = res;
    return fakeStatus;
  } else {
    return new Response(
        "Invalid mode: " + res.mode + "; valid modes: " + fakeStatus["validModes"],
        {status: 400});
  }
});
fetchMock.post("/dist/arb-status-sk.html/json/strategy",
    function(url: string, opts: fetchMock.MockOptions) {
  const res = <Strategy>JSON.parse(<string><unknown>opts.body);
  res["user"] = "you@google.com";
  const validStrategy = fakeStatus["validStrategies"].indexOf(res.strategy);
  if (validStrategy >= 0) {
    fakeStatus["strategy"] = res;
    return fakeStatus;
  } else {
    return new Response(
        "Invalid strategy: " + res.strategy + "; valid strategies: " +
            fakeStatus["validStrategies"],
        {status: 400});
  }
});
fetchMock.post("/dist/arb-status-sk.html/json/unthrottle", {});
fetchMock.post("/dist/arb-status-sk.html/json/manual",
    function(url: string, opts: fetchMock.MockOptions) {
  const req = <ManualRollRequest>JSON.parse(<string><unknown>opts.body);
  req.requester = "you@google.com";
  req.result = manualRollResults[manualRollResult++ % manualRollResults.length];
  req.rollerName = "skia-autoroll";
  req.status = "PENDING";
  req.timestamp = "2017-08-28T03:51:10Z";
  req.url = req.result == "" ? "" : "https://fake.google.com";
  if (!fakeStatus["manualRequests"]) {
    fakeStatus["manualRequests"] = [];
  }
  fakeStatus["manualRequests"].push(req);
  return req;
});

import './index.ts';