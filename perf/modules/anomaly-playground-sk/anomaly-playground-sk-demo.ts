import fetchMock from 'fetch-mock';

import './index';

fetchMock.post('/_/start', 200);
fetchMock.post('/_/update', 200);
fetchMock.post('/_/shortcut/update', () => ({ id: 'test-shortcut' }));
fetchMock.post('/_/frame/start', () => ({ id: 'test-frame-id' }));
