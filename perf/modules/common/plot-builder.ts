// Contains functions to create plot data.

import '@google-web-components/google-chart/';

export interface DataPoint {
  x: number | Date;
  y: number;
  anomaly: boolean;
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

export function DrawSummaryChart(
  canvas: HTMLElement,
  chartData: ChartData,
  width: number,
  height: number,
  style: CSSStyleDeclaration
) {
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

  const format =
    chartData.chartAxisFormat === ChartAxisFormat.Commit ? '#' : 'MMM dd, yy';
  const options: google.visualization.LineChartOptions = {
    width: width,
    height: height,
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

  const dataForChart = google.visualization.arrayToDataTable(rows);
  const chart = new google.visualization.LineChart(canvas);
  chart.draw(dataForChart, options);
}
