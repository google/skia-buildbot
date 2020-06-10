import './index';
import { WordCloudSk } from './word-cloud-sk';

let items = [
  { value: 'arch=x86', percent: 100 },
  { value: 'config=565', percent: 60 },
  { value: 'config=8888', percent: 50 },
  { value: 'arch=arm', percent: 20 },
];

document.querySelectorAll<WordCloudSk>('word-cloud-sk').forEach((wc) => {
  wc.items = items;
});
