import { fromParamSet } from '../../../infra-sk/modules/query';
import { Params, ParamSet, TraceFormat } from '../json';
import { makeKey } from '../paramtools';

import '../window/window';

// TraceFormatter provides an interface to format trace details.
export interface TraceFormatter {
  // formatTrace returns a formatted string for the given param set
  formatTrace(params: Params): string;

  //formatQuery returns a formatted query string for the given trace string.
  formatQuery(trace: string): string;
}

// DefaultTraceFormatter provides default trace formatting
export class DefaultTraceFormatter implements TraceFormatter {
  formatTrace(params: Params): string {
    return `Trace ID: ${makeKey(params)}`;
  }

  formatQuery(_: string): string {
    return '';
  }
}

// ChromeTraceFormatter formats the trace details for chrome instances
export class ChromeTraceFormatter implements TraceFormatter {
  // TODO(eduardoyap): Remove hardcodings and implement more flexible format.
  private readonly keys = [
    'master',
    'bot',
    'benchmark',
    'test',
    'subtest_1',
    'subtest_2',
    'subtest_3',
  ];

  // formatTrace formats the param set in the form
  // master/bot/benchmark/test/subtest_1/subtest_2/subtest_3
  formatTrace(params: Params): string {
    const resultParts = [];
    for (const key of this.keys) {
      if (key in params) {
        resultParts.push(params[key]);
      }
    }

    return resultParts.join('/');
  }

  // formatQuery converts a trace string to a query string.
  // The trace string is split by '/' and the values are assigned to the keys
  // defined in the keys array. Each key-value pair is then converted to a
  // query parameter in the format "key=value".
  formatQuery(trace: string): string {
    const parts = trace.split('/');
    const paramSet: ParamSet = ParamSet({});
    for (let i = 0; i < parts.length; i++) {
      if (i < this.keys.length) {
        paramSet[this.keys[i]] = [parts[i]];
      }
    }
    return fromParamSet(paramSet);
  }
}

// traceFormatterRecords specifies TraceFormat to TraceFormatter mapping records
const traceFormatterRecords: Record<TraceFormat, TraceFormatter> = {
  '': new DefaultTraceFormatter(),
  chrome: new ChromeTraceFormatter(),
};

// GetTraceFormatter returns a TraceFormatter instance based on config.
export function GetTraceFormatter(): TraceFormatter {
  return traceFormatterRecords[window.perf.trace_format];
}
