const STATISTIC_VALUES = ['avg', 'count', 'max', 'min', 'std', 'sum', 'p50', 'p90', 'p95', 'p99'];

export interface ParsedTestPath {
  bot: string;
  configuration: string;
  benchmark: string;
  chart: string;
  statistic: string;
  story: string;
}

/**
 * Parses a standard Perf testPath (e.g. "ChromiumPerf/bot/benchmark/chart/story")
 * into its constituent components for Pinpoint bisect and try job requests.
 */
export function parseTestPath(testPath: string): ParsedTestPath {
  const parameters = testPath.split('/');
  const bot = parameters[0] || '';
  const configuration = parameters[1] || '';
  const benchmark = parameters[2] || '';

  const test = parameters.length > 3 ? parameters[3] : parameters.at(-2) || '';
  const parts = test.split(':');
  const tail = parts.pop();

  let chart = test;
  let statistic = '';
  if (tail !== undefined) {
    chart = STATISTIC_VALUES.includes(tail) ? parts.join('_') : test;
    statistic = STATISTIC_VALUES.includes(tail) ? tail : '';
  }

  const story = (parameters.slice(3).pop() || '').replace(/:/g, '_');

  return {
    bot,
    configuration,
    benchmark,
    chart,
    statistic,
    story,
  };
}
