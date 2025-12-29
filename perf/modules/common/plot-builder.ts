// Contains functions to create plot data.

import { MISSING_DATA_SENTINEL } from '../const/const';
import { Anomaly, ColumnHeader, TraceSet } from '../json';

// Google chart's default color palette, two shades down
export const defaultColors = [
  '#e36041', // rgb(227, 96, 65)
  '#3fab46', // rgb(63, 171, 70)
  '#ad32ad', // rgb(173, 50, 173)
  '#6264bc', // rgb(98, 100, 188)
  '#32add1', // rgb(50, 173, 209)
  '#e36992', // rgb(227, 105, 146)
  '#ffad32', // rgb(255, 173, 50)
  '#84bb32', // rgb(132, 187, 50)
  '#c65757', // rgb(184, 46, 46)
  '#5a82aa', // rgb(90, 130, 170)
  '#ad69ad', // rgb(173, 105, 173)
  '#4ebbad', // rgb(78, 87, 173)
  '#bbbb40', // rgb(187, 187, 64)
  '#845bd6', // rgb(132, 91, 214)
  '#eb8f32', // rgb(235, 143, 50)
  '#5b84d6', // rgb(91, 132, 214)
  '#a23838', // rgb(162, 56, 56)
  '#5aa781', // rgb(90, 167, 129)
  '#5574A6', // rgb(85, 116, 166)
  '#3B3EAC', // rgb(59, 62, 172)
];

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
export const convertFromDataframe = (
  df: {
    traceset: TraceSet;
    header: (ColumnHeader | null)[] | null;
  } | null,
  domain: 'commit' | 'date' | 'both' = 'commit',
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
  if (domain === 'commit' || domain === 'both') {
    firstRow.push({ type: 'number', role: 'domain', label: 'Commit Position' });
  }
  if (domain === 'date' || domain === 'both') {
    firstRow.push({ type: 'date', role: 'domain', label: 'Date' });
  }
  keys.forEach((k) => firstRow.push(k));

  const rows: any[][] = [firstRow];
  const isAllMissing = keys.map((k) => df!.traceset[k].every((v) => v === MISSING_DATA_SENTINEL));

  df!.header?.forEach((column, idx) => {
    const row: any[] = [];
    if (domain === 'commit' || domain === 'both') {
      row.push(column!.offset);
    }
    if (domain === 'date' || domain === 'both') {
      row.push(new Date(column!.timestamp * 1000));
    }
    keys.forEach((k, keyIndex) => {
      const val = df!.traceset[k][idx];
      if (isAllMissing[keyIndex] && idx === df!.header!.length - 1) {
        row.push(0);
      } else {
        row.push(val === MISSING_DATA_SENTINEL ? null : val);
      }
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
  domain: string,
  yAxis: string | undefined,
  showZero: boolean = false
): google.visualization.LineChartOptions {
  // The X axis can support either commit, or dates. Change the format
  // based on the current chart's format.
  const gridlineColor = style.getPropertyValue('--md-sys-color-inverse-on-surface');
  const minAxisValue: number | undefined = showZero ? 0 : undefined;
  const format = domain === 'commit' ? '#' : 'M/d/yy';
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
    annotations: {
      style: 'line',
    },
    tooltip: { trigger: 'none' },
    pointSize: 3,
    dataOpacity: 0.7,
    titleTextStyle: { color: style.color },
    hAxis: {
      titleTextStyle: {
        color: style.getPropertyValue('--plot-axes-title-color'),
      },
      textPosition: 'out',
      textStyle: {
        color: style.color,
      },
      gridlines: {
        color: gridlineColor,
      },
      minorGridlines: {
        color: gridlineColor,
      },
      format: format,
    },
    vAxis: {
      minValue: minAxisValue,
      title: yAxis,
      titleTextStyle: {
        color: style.getPropertyValue('--plot-axes-title-color'),
      },
      textPosition: 'out',
      textStyle: { color: style.color },
      gridlines: {
        color: gridlineColor,
      },
      minorGridlines: {
        color: gridlineColor,
      },
      viewWindowMode: 'maximized',
    },
    // define clearance to prevent axis labels and data from getting clipped
    chartArea: {
      left: 120,
      right: 10,
      top: 5,
      bottom: 25,
    },
    colors: defaultColors,
    legend: {
      position: 'none',
    },
    backgroundColor: style.getPropertyValue('--plot-background-color-sk'),
    series: {},
    crosshair: {
      trigger: 'both',
      focused: {
        opacity: 0.5,
      },
    },
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
        fontSize: 8,
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
    tooltip: {
      trigger: 'none',
    },
    chartArea: {
      width: '100%',
      bottom: 25, // prevents commit positions from being clipped
      top: 0,
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

/**
 * Returns a consistent color for a given trace name.
 *
 * @param {string} traceName - The name of the trace.
 * @return {string} The hex color string.
 */
export function getTraceColor(traceName: string): string {
  let hash = 0;
  for (let i = traceName.length - 1; i >= 0; i--) {
    // eslint-disable-next-line no-bitwise
    hash = (hash << 5) - hash + traceName.charCodeAt(i);
    // eslint-disable-next-line no-bitwise
    hash |= 0;
  }
  hash = Math.abs(hash);
  return defaultColors[hash % defaultColors.length];
}
