// Contains functions to create plot data.

import '@google-web-components/google-chart/';

export interface DataPoint {
  x: number | Date;
  y: number;
}

export interface ChartData {
  xLabel: string;
  yLabel: string;
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

  let background = style.getPropertyValue('--background');
  if (background === '') {
    background = 'white';
  }
  const options: google.visualization.LineChartOptions = {
    legend: 'none',
    width: width,
    height: height,
    hAxis: {
      textPosition: 'none',
      gridlines: {
        color: 'transparent',
      },
    },
    vAxis: {
      textPosition: 'none',
      gridlines: {
        color: 'transparent',
      },
    },
    chartArea: {
      width: '100%',
      height: '100%',
    },
    backgroundColor: background,
    colors: [style.color],
  };
  const chart = new google.visualization.LineChart(canvas);
  chart.draw(dataForChart, options);
}
