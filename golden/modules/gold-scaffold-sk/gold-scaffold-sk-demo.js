import './index';
import { testOnlySetSettings } from '../settings';
import { $$ } from 'common-sk/modules/dom';

testOnlySetSettings({
  title: 'Skia Public',
});
$$('gold-scaffold-sk')._render(); // pick up title from settings.
