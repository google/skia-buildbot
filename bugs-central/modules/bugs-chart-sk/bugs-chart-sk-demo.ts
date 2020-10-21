import './index';
import fetchMock from 'fetch-mock';

const chartData = [
  ['date', 'col1', 'col2'],
  ['2020-10-01', 14, 21],
  ['2020-10-02', 32, 24],
];

fetchMock.get('/_/get_chart_data?client=Android&source=&query=&type=open', chartData);
fetchMock.get('/_/get_chart_data?client=Android&source=Github&query=&type=slo', chartData);

customElements.whenDefined('bugs-chart-sk').then(() => {
  // Insert the elements later, which should given enough time for fetchMock to be in place.
  const page1 = document.createElement('bugs-chart-sk');
  page1.setAttribute('chart_type', 'open');
  page1.setAttribute('chart_title', 'Bug Count');
  page1.setAttribute('client', 'Android');
  page1.setAttribute('source', '');
  page1.setAttribute('query', '');

  const page2 = document.createElement('bugs-chart-sk');
  page2.setAttribute('chart_type', 'slo');
  page2.setAttribute('chart_title', 'SLO Violations');
  page2.setAttribute('client', 'Android');
  page2.setAttribute('source', 'Github');
  page2.setAttribute('query', '');


  document
    .querySelector('h1')!
    .insertAdjacentElement('afterend', page1)!
    .insertAdjacentElement('afterend', page2);
});
