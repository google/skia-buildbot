import fetchMock from 'fetch-mock';
import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { GetClientCountsResponse, StatusData } from '../../../bugs-central/modules/json';

fetchMock.getOnce('https://bugs-central.skia.org/get_client_counts', <GetClientCountsResponse>{
  clients_to_status_data: {
    Android: <StatusData>{
      untriaged_count: 10,
      link: 'www.test-link.com/test1',
    },
    Chromium: <StatusData>{
      untriaged_count: 23,
      link: 'www.test-link.com/test2',
    },
    Skia: <StatusData>{
      untriaged_count: 104,
      link: 'www.test-link.com/test3',
    },
  },
});
const el = document.createElement('bugs-status-sk');
document.querySelector('#container')?.appendChild(el);
