// Contains functions to create plot data.

import '@google-web-components/google-chart/';

export interface DataPoint {
  x: number | Date;
  y: number;
}

export enum ChartAxisFormat {
  Commit,
  Date,
}

export interface ChartData {
  xLabel: string;
  yLabel: string;
  chartAxisFormat: ChartAxisFormat;
  data: DataPoint[];
}

export function DrawSummaryChart(
  canvas: HTMLElement,
  chartData: ChartData,
  width: number,
  height: number,
  style: CSSStyleDeclaration
) {
  const data: [any] = [[chartData.xLabel, chartData.yLabel]];
  chartData.data.forEach((datapoint) => {
    data.push([datapoint.x, datapoint.y]);
  });

  const dataForChart = google.visualization.arrayToDataTable(data);
  const format =
    chartData.chartAxisFormat === ChartAxisFormat.Commit ? '#' : 'MMM dd, yy';
  const options: google.visualization.LineChartOptions = {
    width: width,
    height: height,
    hAxis: {
      textPosition: 'out',
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
  const chart = new google.visualization.LineChart(canvas);
  chart.draw(dataForChart, options);
}
