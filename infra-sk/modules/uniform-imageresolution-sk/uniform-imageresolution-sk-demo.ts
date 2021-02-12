import { $, $$ } from 'common-sk/modules/dom';
import './index';
import { UniformImageresolutionSk } from './uniform-imageresolution-sk';

$$('#apply')!.addEventListener('click', () => {
  const uniforms: number[] = [0, 0, 0];
  $<UniformImageresolutionSk>('uniform-imageresolution-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
});
