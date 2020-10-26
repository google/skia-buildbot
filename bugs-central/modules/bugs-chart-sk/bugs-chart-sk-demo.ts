import './index';

const chartData = [
  ['date', 'col1', 'col2'],
  ['2020-10-01', 14, 21],
  ['2020-10-02', 32, 24],
];

customElements.whenDefined('bugs-chart-sk').then(() => {
  const page1 = document.createElement('bugs-chart-sk');
  page1.setAttribute('chart_type', 'open');
  page1.setAttribute('chart_title', 'Bug Count');
  page1.setAttribute('data', JSON.stringify(chartData));

  const page2 = document.createElement('bugs-chart-sk');
  page2.setAttribute('chart_type', 'slo');
  page2.setAttribute('chart_title', 'SLO Violations');
  page2.setAttribute('data', JSON.stringify(chartData));

  const page3 = document.createElement('bugs-chart-sk');
  page3.setAttribute('chart_type', 'untriaged');
  page3.setAttribute('chart_title', 'Untriaged Bugs');
  page3.setAttribute('data', JSON.stringify(chartData));

  document
    .querySelector('h1')!
    .insertAdjacentElement('afterend', page1)!
    .insertAdjacentElement('afterend', page2)!
    .insertAdjacentElement('afterend', page3);
});
