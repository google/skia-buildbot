import './index';
import { ClusterSummary2Sk } from './cluster-summary2-sk';

window.Login = Promise.resolve(
  {
    Email: 'user@google.com',
    LoginURL: 'https://accounts.google.com/',
  },
);

ClusterSummary2Sk._lookupCids = () => new Promise(((resolve) => {
  resolve([{
    offset: 24748, author: 'msarett@google.com', message: '313c463 - Safely handle unsupported color xforms in SkCodec', url: 'https://skia.googlesource.com/skia/+show/313c4635e3f1005e6807f5b0ad52805f30902d66', ts: 1476984695,
  }]);
}));
