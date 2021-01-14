import './index.ts';
import './particles-config-sk-demo.css';
import { $$ } from 'common-sk/modules/dom';
import { ParticlesConfigSk } from './particles-config-sk';

$$('#open')!.addEventListener('click', async () => {
  try {
    const config = await $$<ParticlesConfigSk>('particles-config-sk')!.show({
      body: null,
      width: 500,
      height: 500,
    });
    $$('#results')!.textContent = JSON.stringify(config, null, '  ');
  } catch (err) {
    $$('#results')!.textContent = err;
  }
});
