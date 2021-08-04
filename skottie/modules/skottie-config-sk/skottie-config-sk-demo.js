import './index';
import { $$ } from 'common-sk/modules/dom';

(function() {
  const ele = $$('#given');
  const msg = $$('#msg');
  ele.state = {
    filename: 'foo.json',
    lottie: {},
    width: 128,
    height: 128,
    fps: 60,
  };

  const display = (e) => {
    msg.innerHTML = `${e.type}
${JSON.stringify(e.detail)}
`;
  };

  document.addEventListener('skottie-selected', display);
  document.addEventListener('cancelled', display);
}());
