import './index';

document.addEventListener('DOMContentLoaded', () => {
  customElements.whenDefined('tooltip-sk').then(() => {
    (document.querySelector('#testctrl')! as HTMLInputElement).focus();
  });
});
