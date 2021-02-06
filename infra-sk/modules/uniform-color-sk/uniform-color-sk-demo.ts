import './index';

import { $$ } from 'common-sk/modules/dom';
import './index';
import { $ } from 'common-sk/modules/dom';
import { UniformColorSk } from './uniform-color-sk';

$$('#apply')!.addEventListener('click', () => {
  const uniforms = new Float32Array(6);
  $<UniformColorSk>('uniform-color-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
});

$$<UniformColorSk>('#secondColor')!.uniform = {
  name: 'particle',
  rows: 1,
  columns: 3,
  slot: 3,
};
