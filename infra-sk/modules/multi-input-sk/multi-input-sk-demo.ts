import './index';

document.querySelector('multi-input-sk')!.addEventListener('change', (e) => {
  console.log(e);
  const pre = document.createElement('pre');
  pre.innerText = JSON.stringify(e, null, '  ');
  document.querySelector('#events')!.appendChild(pre);
});
