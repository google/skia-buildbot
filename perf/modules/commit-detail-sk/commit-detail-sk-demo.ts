import './index';
import { CommitDetailSk } from './commit-detail-sk';

Date.now = () => Date.parse('2020-03-22T00:00:00.000Z');

document.querySelectorAll<CommitDetailSk>('commit-detail-sk').forEach((ele) => {
  ele.cid = {
    hash: 'e699a3a2373bc4c2a4bfa93c7af8602cb15f2d1d',
    url: 'https://skia.googlesource.com/skia/+show/e699a3a2373bc4c2a4bfa93c7af8602cb15f2d1d',
    message: 'Roll third_party/externals/swiftshader 522d5121905',
    author: '',
    ts: 0,
    offset: 0,
  };
  ele.querySelector('div')!.click();
});
