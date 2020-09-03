import fetchMock from 'fetch-mock';
import './index';
import { TestSrcSk } from './test-src-sk';

const value = 'Hello world!';
const url = '/some-text-endpoint';
fetchMock.get(url, value);
document.querySelector<TestSrcSk>('test-src-sk')!.src = url;
