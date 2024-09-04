import { MISSING_DATA_SENTINEL } from '../const/const';
import { Anomaly, ColumnHeader } from '../json';
import { AnomalyData } from '../plot-simple-sk/plot-simple-sk';
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

// findMatchingAnomaly will search if the (trace, x and y coordinate) is an
// anomaly from the anomaly set.
function findMatchingAnomaly(
  traceKey: string,
  current_x: number | Date,
  current_y: number,
  anomalies: { [key: string]: AnomalyData[] }
): Anomaly | null {
  const anomalyTraceKeys = Object.keys(anomalies);
  if (!(traceKey in anomalyTraceKeys)) {
    return null;
  }

  for (let x = 0; x < anomalies[traceKey].length; x++) {
    const ad = anomalies[traceKey][x];
    if (ad.x === current_x && ad.y === current_y) {
      return ad.anomaly;
    }
  }

  return null;
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
  chartAxisFormat: ChartAxisFormat,
  anomalies: { [key: string]: AnomalyData[] }
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
  const anomalyTraceKeys = Object.keys(anomalies);
  traceKeys.forEach((key) => {
    const trace = traceSet[key];
    const traceDataPoints: DataPoint[] = [];
    for (let i = 0; i < trace.length; i++) {
      const x_coordinate = xLabels[i];
      const y_coordinate = trace[i];

      // Anomalies are formatted to a map of trace_id: AnomalyData[], where each AnomalyData
      // defines it's own (x, y) coordinates, and details of the anomaly (wrapped in an Anomaly)
      // interface. Adding this loop should not impact performance significantly because
      // it becomes O(n * (m + x)) where n = # traces, m = # y axis coordinates and x = #
      // of anomalies for the trace.
      const anomaly_data = findMatchingAnomaly(
        key,
        x_coordinate,
        y_coordinate,
        anomalies
      );

      // The MISSING_DATA_SENTINEL const is used to define missing data points
      // at the given x value in the trace. We should ignore these points when
      // we create the chart data since the charts library will automatically handle
      // this scenario.
      if (trace[i] !== MISSING_DATA_SENTINEL) {
        const dataPoint: DataPoint = {
          x: x_coordinate,
          y: y_coordinate,
          anomaly: anomaly_data,
        };
        traceDataPoints.push(dataPoint);
      }
    }

    chartData.lines[key] = traceDataPoints;
  });

  return chartData;
}
