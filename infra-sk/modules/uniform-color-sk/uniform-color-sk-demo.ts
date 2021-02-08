import './index';

import { $$ } from 'common-sk/modules/dom';
import './index';
import { $ } from 'common-sk/modules/dom';
import { UniformColorSk } from './uniform-color-sk';

$$('#apply')!.addEventListener('click', () => {
  const uniforms: number[] = [0, 0, 0, 0, 0, 0, 0];
  $<UniformColorSk>('uniform-color-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
});

$$<UniformColorSk>('#withAlphaColor')!.uniform = {
  name: 'iColorWithAlpha',
  rows: 1,
  columns: 4,
  slot: 3,
};
