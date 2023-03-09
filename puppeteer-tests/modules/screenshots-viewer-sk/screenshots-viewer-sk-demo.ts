import fetchMock from 'fetch-mock';
import { rpcResponse } from './demo_data';
import './index';
import { ScreenshotsViewerSk } from './screenshots-viewer-sk';

fetchMock.get('/rpc/get-screenshots', rpcResponse);

document.body.appendChild(new ScreenshotsViewerSk());
