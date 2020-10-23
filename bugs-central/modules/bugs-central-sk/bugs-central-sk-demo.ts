import './index';
import fetchMock from 'fetch-mock';
import { BugsCentralSk } from './bugs-central-sk';

const testClientsTestMap = {
  clients: {
    Android: {
      Github: {
        'test query': true,
      },
    },
  },
};

const testIssueCounts = {
  open_count: 212,
  unassigned_count: 12,

  p0_count: 2,
  p1_count: 10,
  p2_count: 50,
  p3_count: 50,
  p4_count: 50,
  p5_count: 30,
  p6_count: 20,

  p0_slo_count: 1,
  p1_slo_count: 2,
  p2_slo_count: 3,
  p3_slo_count: 4,

  query_link: 'http://test_query_link/',
};

const chartData = [
  ['date', 'col1', 'col2'],
  ['2020-10-01', 14, 21],
  ['2020-10-02', 32, 24],
];

fetchMock.post('/_/get_clients_sources_queries', () => testClientsTestMap);
fetchMock.post('/_/get_issue_counts', () => testIssueCounts);
fetchMock.get('/_/get_chart_data?client=Android&source=Github&query=test%20query&type=open', chartData);
fetchMock.get('/_/get_chart_data?client=Android&source=Github&query=test%20query&type=slo', chartData);
fetchMock.get('/_/get_chart_data?client=Android&source=Github&query=test%20query&type=untriaged', chartData);

customElements.whenDefined('bugs-central-sk').then(() => {
  // Insert the element later, which should given enough time for fetchMock to be in place.
  document
    .querySelector('h1')!
    .insertAdjacentElement(
      'afterend',
      document.createElement('bugs-central-sk'),
    );

  const elems = document.querySelectorAll<BugsCentralSk>('bugs-central-sk')!;
  elems.forEach((el) => {
    el.state = {
      client: 'Android',
      source: 'Github',
      query: 'test query',
    };
  });
});
