import { MISSING_DATA_SENTINEL } from '../const/const';
import { ColumnHeader } from '../json';
import { ChartAxisFormat, ChartData, DataPoint } from './plot-builder';

/**
 * GetSelectionDateIndicesFromColumnHeader returns the indices of the start and end date
 * from the column header dates.
 * @param header ColumnHeader object
 * @param start start date
 * @param end end date
 * @returns Array of length 2 containing [startIndex, endIndex]
 */
export function GetSelectionDateIndicesFromColumnHeader(
  header: (ColumnHeader | null)[],
  start: Date,
  end: Date
): number[] {
  let startIndex = 0;
  let endIndex = 0;
  for (let i = 0; i < header.length; i++) {
    const currentCommitDate = new Date(header[i]!.timestamp * 1000);
    if (startIndex === 0 && start < currentCommitDate) {
      if (i > 0) {
        startIndex = i - 1;
      }
    }
    if (endIndex === 0 && end < currentCommitDate) {
      endIndex = i;
    }

    if (startIndex > 0 && endIndex > 0) {
      break;
    }
  }
  // This is likely because the summary selection has
  // selected the extreme end. Let's default it to the
  // last index.
  if (endIndex === 0) {
    endIndex = header.length - 1;
  }

  return [startIndex, endIndex];
}

/**
 * GetSelectionCommitIndicesFromColumnHeader returns the indices of the start and end commits
 * from the column header.
 * @param header ColumnHeader object.
 * @param start start commit.
 * @param end end commit.
 * @returns Array of length 2 containing [startIndex, endIndex].
 */
export function GetSelectionCommitIndicesFromColumnHeader(
  header: (ColumnHeader | null)[],
  start: number,
  end: number
): number[] {
  let startIndex = 0;
  let endIndex = 0;
  for (let i = 0; i < header.length; i++) {
    const currentCommit = header[i]!.offset;
    if (startIndex === 0 && start < currentCommit) {
      if (i > 0) {
        startIndex = i - 1;
      }
    }
    if (endIndex === 0 && end < currentCommit) {
      endIndex = i;
    }

    if (startIndex > 0 && endIndex > 0) {
      break;
    }
  }

  // This is likely because the summary selection has
  // selected the extreme end. Let's default it to the
  // last index.
  if (endIndex === 0) {
    endIndex = header.length - 1;
  }

  return [startIndex, endIndex];
}

/**
 * Create the chart data object from the traceSet.
 * @param traceSet The traceset input.
 * @param xLabels Labels for the xAxis. The length of the labels should be equal to the length
 * of the values array for each trace.
 * @returns ChartData for the provided input.
 */
export function CreateChartDataFromTraceSet(
  traceSet: { [key: string]: number[] },
  xLabels: (number | Date)[],
  chartAxisFormat: ChartAxisFormat
): ChartData {
  const chartData: ChartData = {
    lines: {},
    xLabel: chartAxisFormat.toString(),
    yLabel: 'Value',
    chartAxisFormat: chartAxisFormat,
    start: xLabels[0],
    end: xLabels[xLabels.length - 1],
  };

  const traceKeys = Object.keys(traceSet);
  traceKeys.forEach((key) => {
    const trace = traceSet[key];
    const traceDataPoints: DataPoint[] = [];
    for (let i = 0; i < trace.length; i++) {
      // The MISSING_DATA_SENTINEL const is used to define missing data points
      // at the given x value in the trace. We should ignore these points when
      // we create the chart data since the charts library will automatically handle
      // this scenario.
      if (trace[i] !== MISSING_DATA_SENTINEL) {
        const dataPoint: DataPoint = {
          x: xLabels[i],
          y: trace[i],
          anomaly: false,
        };
        traceDataPoints.push(dataPoint);
      }
    }

    chartData.lines[key] = traceDataPoints;
  });

  return chartData;
}
