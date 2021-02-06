import { $, $$ } from 'common-sk/modules/dom';
import './index';
import { UniformGenericSk } from './uniform-generic-sk';

$$<UniformGenericSk>('#float2')!.uniform = {
  name: 'iLocation',
  columns: 2,
  rows: 1,
  slot: 1,
};

$$<UniformGenericSk>('#float3x3')!.uniform = {
  name: 'iMatrix',
  columns: 3,
  rows: 3,
  slot: 3,
};

$$('#apply')!.addEventListener('click', () => {
  const uniforms = new Float32Array(1 + 2 + 9 + 1);
  $<UniformGenericSk>('uniform-generic-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
});
