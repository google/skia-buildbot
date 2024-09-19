// Contains functions to create plot data.

import { Anomaly } from '../json';

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

// ConvertMainData takes the chartData, formats it for Google Chart library.
// The primary difference between this and CovertData is the inclusion of
// anomalies.
export function convertMainData(chartData: ChartData) {
  // The data for the plot must be of the format:
  // [
  //   [x-axis label, line1 label, line2 label, ...],
  //   [x_value, line1 value, line2 value, ...], // first point
  //   [x_value, line1 value, line2 value, ...], // second point
  //   ...
  // ]

  // add all columns to data first.
  const columns: [any] = [chartData.xLabel];
  const lineKeys = Object.keys(chartData.lines);
  lineKeys.forEach((key) => {
    columns.push(key);
    // if the point requires some custom styling (ie/ to change the point to
    // an anomaly), it needs a style column. So, we add a style column for each
    // "trace". If it doesn't need anything, the value for this column should
    // be null.
    // https://developers.google.com/chart/interactive/docs/roles#what-roles-are-available
    columns.push({ type: 'string', role: 'style' });
  });

  // then add the rows. first row defines the columns.
  const rows: [any] = [columns];

  const rowCount = chartData.lines[lineKeys[0]].length;
  for (let i = 0; i < rowCount; i++) {
    // Add the xValue which is the same for all lines
    const row = [chartData.lines[lineKeys[0]][i].x] as any[];
    lineKeys.forEach((key) => {
      // For each line, add the y value for datapoint at index i.
      row.push(chartData.lines[key][i].y);
      if (chartData.lines[key][i].anomaly) {
        row.push('point { size: 10; shape-type: triangle; }');
      } else {
        row.push(null);
      }
    });
    rows[i + 1] = row;
  }

  return rows;
}

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
  chartData: ChartData
): google.visualization.LineChartOptions {
  // The X axis can support either commit, or dates. Change the format
  // based on the current chart's format.
  const format =
    chartData.chartAxisFormat === ChartAxisFormat.Commit ? '#' : 'MM/dd/yy';
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
      gridlines: {
        color: 'transparent',
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
      width: '80%',
      height: '85%',
    },
    explorer: {
      axis: 'horizontal',
      actions: ['dragToZoom', 'rightClickToReset'],
    },
    legend: { position: 'top' },
    backgroundColor: style.backgroundColor,
    series: {},
  };
}

export function SummaryChartOptions(
  style: CSSStyleDeclaration,
  chartData: ChartData
): google.visualization.LineChartOptions {
  const format =
    chartData.chartAxisFormat === ChartAxisFormat.Commit ? '#' : 'MMM dd, yy';
  return {
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
    backgroundColor: style.backgroundColor,
    colors: [style.color],
  };
}
