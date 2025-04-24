import { fromParamSet } from '../../../infra-sk/modules/query';
import { Params, ParamSet, TraceFormat } from '../json';
import { makeKey } from '../paramtools';

import '../window/window';

const STATISTIC_SUFFIX_TO_VALUE_MAP = new Map<string, string>([
  ['avg', 'value'],
  ['count', 'count'],
  ['max', 'max'],
  ['min', 'min'],
  ['std', 'error'],
  ['sum', 'sum'],
]);

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
    // We are using Chromeperf's test path to generate a query for Skia traces.
    // If default stat value is removed from the query, we should add the stat value in
    // paramset to avoid loading 6x traces. E.g., when the trace's 'test' value has no
    // stat suffix, it will have six 'stat' values. Those 'stat' values will be used in
    // the future Skia. But in the context of Chromeperf, it means the average, and thus
    // we should add the 'stat' value for trace query.
    // This ad hoc logic is specific to the Chromeperf style test_path used in
    // Chromeperf anomalies. It is no longer needed when Chromeperf is deprecated.
    if (window.perf.remove_default_stat_value) {
      const testValue = paramSet['test'][0];
      const testParts = testValue.split('_');
      const suffix = testParts.pop();
      if (suffix !== undefined) {
        if (STATISTIC_SUFFIX_TO_VALUE_MAP.has(suffix)) {
          // use the suffix to decide the stat value, and use the trimmed test name.
          paramSet['test'] = [testParts.join('_')];
          paramSet['stat'] = [STATISTIC_SUFFIX_TO_VALUE_MAP.get(suffix)!];
        } else {
          // apply the default stat value.
          paramSet['stat'] = ['value'];
        }
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
