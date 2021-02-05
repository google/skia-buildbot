import { $$ } from 'common-sk/modules/dom';
import './index';
import { UniformSliderSk } from './uniform-slider-sk';
import { $ } from 'common-sk/modules/dom';

$$('#apply')!.addEventListener('click', () => {
  const uniforms = new Float32Array(3);
  $<UniformSliderSk>('uniform-slider-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
});

$$<UniformSliderSk>('#rate')!.uniform = {
  name: 'u_rate',
  columns: 1,
  rows: 1,
  slot: 1,
};
