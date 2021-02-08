import { $, $$ } from 'common-sk/modules/dom';
import './index';
import { UniformDimensionsSk } from './uniform-dimensions-sk';

$$('#apply')!.addEventListener('click', () => {
  // Pick a larger than needed uniforms size to show we don't affect the other uniform values.
  const uniforms = [0, 0, 0, 0, 0];
  $<UniformDimensionsSk>('uniform-dimensions-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
});
