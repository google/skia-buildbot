import { Params, TraceFormat } from '../json';
import { makeKey } from '../paramtools';

import '../window/window';

// TraceFormatter provides an interface to format trace details.
export interface TraceFormatter {
  // formatTrace returns a formatted string for the given param set
  formatTrace(params: Params): string;
}

// DefaultTraceFormatter provides default trace formatting
export class DefaultTraceFormatter implements TraceFormatter {
  formatTrace(params: Params): string {
    return `Trace ID: ${makeKey(params)}`;
  }
}

// ChromeTraceFormatter formats the trace details for chrome instances
export class ChromeTraceFormatter implements TraceFormatter {
  // formatTrace formats the param set in the form
  // master/bot/benchmark/test/subtest_1/subtest_2/subtest_3
  formatTrace(params: Params): string {
    const keys = [
      'master',
      'bot',
      'benchmark',
      'test',
      'subtest_1',
      'subtest_2',
      'subtest_3',
    ];
    const resultParts = [];
    for (const key of keys) {
      if (key in params) {
        resultParts.push(params[key]);
      }
    }

    return resultParts.join('/');
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
