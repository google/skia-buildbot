// Contains functions to create plot data.

import { MISSING_DATA_SENTINEL } from '../const/const';
import { Anomaly, ColumnHeader, TraceSet } from '../json';

export interface DataPoint {
  x: number | Date;
  y: number;
  anomaly: Anomaly | null;
}

export enum ChartAxisFormat {
  Commit,
  Date,
}

export interface ChartData {
  xLabel: string;
  yLabel: string;
  chartAxisFormat: ChartAxisFormat;
  lines: { [key: string]: DataPoint[] };
  start: number | Date;
  end: number | Date;
}

// convertFromDataframe converts DataFrame to any[][]  that can be plugged into
// GoogleChart.data.
// This function effectively transposes the traceset so that the keys
// (i.e. jetstream2/box2D/etc) become the columns and the individual data points
// are the rows
// TODO(b/362831653): fix legend in the dataframe
export const convertFromDataframe = (
  df: {
    traceset: TraceSet;
    header: (ColumnHeader | null)[] | null;
  } | null,
  domain: 'commit' | 'date' = 'commit',
  traceKey?: string
) => {
  if ((df?.header?.length || 0) === 0) {
    return null;
  }
  if (traceKey && !(traceKey in df!.traceset)) {
    return null;
  }

  const keys = traceKey ? [traceKey] : Object.keys(df!.traceset);

  const firstRow: any[] = [];
  if (domain === 'commit') {
    firstRow.push({ type: 'number', role: 'domain', label: 'Commit Position' });
  } else {
    firstRow.push({ type: 'date', role: 'domain', label: 'Date' });
  }
  keys.forEach((k) => firstRow.push(k));

  const rows: any[][] = [firstRow];
  df!.header?.forEach((column, idx) => {
    const row: any[] = [];
    if (domain === 'commit') {
      row.push(column!.offset);
    } else {
      row.push(new Date(column!.timestamp * 1000));
    }
    keys.forEach((k) => {
      const val = df!.traceset[k][idx];
      row.push(val === MISSING_DATA_SENTINEL ? null : val);
    });
    rows.push(row);
  });
  return rows;
};

export function ConvertData(chartData: ChartData) {
  /*
    The data in the plot needs to be in the following format.
    [
      [x-axis_label, line1_label, line2_label, ...],
      [x_value, line1_value, line2_value, ...], // first point
      [x_value, line1_value, line2_value, ...], // second point
      ...
      ...
    ]
  */
  const columns = [chartData.xLabel];
  const lineKeys = Object.keys(chartData.lines);
  lineKeys.forEach((key) => {
    columns.push(key);
  });
  // The first row needs to be the column names
  const rows: [any] = [columns];
  const rowCount = chartData.lines[lineKeys[0]].length;
  for (let i = 0; i < rowCount; i++) {
    // Add the xValue which is the same for all lines
    const row = [chartData.lines[lineKeys[0]][i].x];
    lineKeys.forEach((key) => {
      // For each line, add the y value for datapoint at index i.
      row.push(chartData.lines[key][i].y);
    });

    rows[i + 1] = row;
  }
  return rows;
}

export function mainChartOptions(
  style: CSSStyleDeclaration,
  domain: string
): google.visualization.LineChartOptions {
  // The X axis can support either commit, or dates. Change the format
  // based on the current chart's format.
  const format = domain === 'commit' ? '#' : 'MM/dd/yy';
  return {
    // interpolateNulls will continue a line from the last known point to the
    // next available if there's nulls inbetween.
    interpolateNulls: true,
    // selectionMode multiple allows you to select multiple points on a plot.
    // we are going to only allow single so that on selection over an area, we
    // can estimate the delta range. note that the default is single.
    selectionMode: 'single',
    // https://developers.google.com/chart/interactive/docs/gallery/areachart
    // Defines how multiple data selections are rolled into the tooltip.
    // for selectionMode: single, this should be none, such that we only allow
    // one tooltip per seelction
    aggregationTarget: 'none',
    tooltip: { trigger: 'selection' },
    pointSize: 2,
    hAxis: {
      textPosition: 'out',
      textStyle: { color: style.color },
      gridlines: {
        color: 'transparent',
      },
      format: format,
    },
    vAxis: {
      textPosition: 'out',
      textStyle: { color: style.color },
      gridlines: {
        color: 'transparent',
      },
      viewWindowMode: 'maximized',
    },
    chartArea: {
      width: '80%',
      height: '85%',
    },
    explorer: {
      axis: 'horizontal',
      actions: ['dragToZoom', 'rightClickToReset'],
    },
    legend: {
      position: 'top',
      textStyle: { color: style.color },
    },
    backgroundColor: style.getPropertyValue('--plot-background-color-sk'),
    series: {},
  };
}

export function SummaryChartOptions(
  style: CSSStyleDeclaration,
  domain: 'commit' | 'date'
): google.visualization.LineChartOptions {
  const format = domain === 'commit' ? '#' : 'MMM dd, yy';
  return {
    interpolateNulls: true,
    curveType: 'function',
    hAxis: {
      textPosition: 'out',
      textStyle: {
        color: style.color,
      },
      gridlines: {
        count: 10,
      },
      format: format,
    },
    vAxis: {
      textPosition: 'out',
      gridlines: {
        color: 'transparent',
      },
      viewWindowMode: 'maximized',
    },
    chartArea: {
      width: '100%',
      height: '50%',
      backgroundColor: {
        stroke: 'black',
        strokeWidth: 1,
      },
    },
    // this value is inherited from plot-google-chart-sk
    backgroundColor: style.getPropertyValue('--plot-background-color-sk'),
    colors: [style.color],
  };
}
