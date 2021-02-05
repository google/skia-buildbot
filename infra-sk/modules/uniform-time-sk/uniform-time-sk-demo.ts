import { $, $$ } from 'common-sk/modules/dom';
import { UniformTimeSk } from './uniform-time-sk';
import './index';

$$('#apply')!.addEventListener('click', () => {
  const uniforms = new Float32Array(2);
  $<UniformTimeSk>('uniform-time-sk')!.forEach((ele) => {
    ele.applyUniformValues(uniforms);
  });
  $$<HTMLPreElement>('#results')!.innerText = uniforms.toString();
});

$$('#run')!.addEventListener('click', () => {
  window.setInterval(() => {
    $<UniformTimeSk>('uniform-time-sk')!.forEach((ele) => {
      ele.render();
    });
  }, 20);
});
