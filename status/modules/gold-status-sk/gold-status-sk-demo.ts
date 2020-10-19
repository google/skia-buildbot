import fetchMock from 'fetch-mock';
import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { StatusResponse } from '../../../golden/modules/rpc_types';

fetchMock.getOnce('https://gold.skia.org/json/v1/trstatus', <StatusResponse>{
  corpStatus: [
    { name: 'canvaskit', untriagedCount: 0 },
    { name: 'colorImage', untriagedCount: 0 },
    { name: 'gm', untriagedCount: 13 },
    { name: 'image', untriagedCount: 0 },
    { name: 'pathkit', untriagedCount: 0 },
    { name: 'skp', untriagedCount: 0 },
    { name: 'svg', untriagedCount: 27 },
  ],
});
const el = document.createElement('gold-status-sk');
document.querySelector('#container')?.appendChild(el);
