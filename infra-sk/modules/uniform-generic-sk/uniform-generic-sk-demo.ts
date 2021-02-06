import { $, $$ } from 'common-sk/modules/dom';
import './index';
import { UniformGenericSk } from './uniform-generic-sk';

$$<UniformGenericSk>('#float2x2')!.uniform = {
  name: 'float2x2',
  columns: 2,
  rows: 2,
  slot: 1,
};

$$<UniformGenericSk>('#float3x3')!.uniform = {
  name: 'float3x3',
  columns: 3,
  rows: 3,
  slot: 5,
};

$$('#apply')!.addEventListener('click', () => {
  const uniforms = new Float32Array(1 + 4 + 9 + 1);
  $<UniformGenericSk>('uniform-generic-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
});
