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
  // Turn on the real clock.
  $<UniformTimeSk>('uniform-time-sk')!.forEach((ele) => {
    ele.dateNow = Date.now;
    ele.time = 0;
  });

  // Update the display periodically.
  window.setInterval(() => {
    $<UniformTimeSk>('uniform-time-sk')!.forEach((ele) => {
      ele.render();
    });
  }, 10);
});

// Start by fixing dateNow for puppeteer tests.
$<UniformTimeSk>('uniform-time-sk')!.forEach((ele) => {
  ele.dateNow = () => 0;
  ele.time = 0;
  ele.render();
});
