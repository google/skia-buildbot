import './index';
import { $$ } from 'common-sk/modules/dom';
import { SkottieConfigSk } from './skottie-config-sk';

(function() {
  const ele = $$<SkottieConfigSk>('#given')!;
  const msg = $$('#msg')!;
  ele.state = {
    assetsFilename: '',
    assetsZip: '',
    filename: 'foo.json',
    lottie: {},
    w: 128,
    h: 128,
  };

  const display = (e: Event) => {
    const detail = (e as CustomEvent).detail;
    msg.innerHTML = `${e.type}
${JSON.stringify(detail)}
`;
  };

  document.addEventListener('skottie-selected', display);
  document.addEventListener('cancelled', display);
}());
