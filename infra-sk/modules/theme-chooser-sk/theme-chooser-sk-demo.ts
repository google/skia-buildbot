import { render, html, TemplateResult } from 'lit/html.js';
import { DARKMODE_LOCALSTORAGE_KEY } from './theme-chooser-sk';
import { CollapseSk } from '../../../elements-sk/modules/collapse-sk/collapse-sk';
import { ToastSk } from '../../../elements-sk/modules/toast-sk/toast-sk';
import { errorMessage } from '../../../elements-sk/modules/errorMessage';
import { gentheme } from '../gentheme';

import '../../../elements-sk/modules/checkbox-sk';
import '../../../elements-sk/modules/collapse-sk';
import '../../../elements-sk/modules/error-toast-sk';
import '../../../elements-sk/modules/multi-select-sk';
import '../../../elements-sk/modules/nav-button-sk';
import '../../../elements-sk/modules/nav-links-sk';
import '../../../elements-sk/modules/radio-sk';
import '../../../elements-sk/modules/select-sk';
import '../../../elements-sk/modules/spinner-sk';
import '../../../elements-sk/modules/tabs-panel-sk';
import '../../../elements-sk/modules/tabs-sk';
import '../../../elements-sk/modules/toast-sk';
import '../../../elements-sk/modules/icons/alarm-icon-sk';
import '../../../elements-sk/modules/icons/check-icon-sk';
import '../../../elements-sk/modules/icons/create-icon-sk';
import '../../../elements-sk/modules/icons/link-icon-sk';
import '../../../elements-sk/modules/icons/menu-icon-sk';
import '../../../elements-sk/modules/icons/warning-icon-sk';
import '../../../elements-sk/modules/icons/expand-less-icon-sk';
import '../../../elements-sk/modules/icons/expand-more-icon-sk';
import '../../../perf/modules/calendar-input-sk';
import '../app-sk';

// eslint-disable-next-line import/first
import './theme-chooser-sk';
import { $, $$ } from '../dom';

// Force the element to use the default mode set in the elements attribute.
window.localStorage.removeItem(DARKMODE_LOCALSTORAGE_KEY);

interface example {
  background: string;
  color: string;
}

const examples: example[] = [
  {
    background: '--background',
    color: '--on-background',
  },
  {
    background: '--primary',
    color: '--on-primary',
  },
  {
    background: '--primary-highlight',
    color: '--on-primary-highlight',
  },
  {
    background: '--secondary',
    color: '--on-secondary',
  },
  {
    background: '--secondary-highlight',
    color: '--on-secondary-highlight',
  },
  {
    background: '--surface',
    color: '--on-surface',
  },
  {
    background: '--surface-1dp',
    color: '--on-surface',
  },
  {
    background: '--surface-2dp',
    color: '--on-surface',
  },
  {
    background: '--surface',
    color: '--primary',
  },
  {
    background: '--surface-1dp',
    color: '--primary',
  },
  {
    background: '--surface-2dp',
    color: '--primary',
  },
  {
    background: '--surface',
    color: '--secondary',
  },
  {
    background: '--surface-1dp',
    color: '--secondary',
  },
  {
    background: '--surface-2dp',
    color: '--secondary',
  },
  {
    background: '--disabled',
    color: '--on-disabled',
  },
  {
    background: '--error',
    color: '--on-error',
  },
  {
    background: '--error-container',
    color: '--on-error-container',
  },
];

const specialty: example[] = [
  {
    background: '--failure',
    color: '--on-failure',
  },
  {
    background: '--failure-alpha',
    color: '--on-failure',
  },
  {
    background: '--warning',
    color: '--on-warning',
  },
  {
    background: '--warning-alpha',
    color: '--on-warning',
  },
  {
    background: '--success',
    color: '--on-success',
  },
  {
    background: '--success-alpha',
    color: '--on-success',
  },
  {
    background: '--unexpected',
    color: '--on-surface',
  },
  {
    background: '--unexpected-alpha',
    color: '--on-surface',
  },
  {
    background: '--untriaged',
    color: '--surface',
  },
  {
    background: '--surface',
    color: '--untriaged',
  },
  {
    background: '--positive',
    color: '--surface',
  },
  {
    background: '--surface',
    color: '--positive',
  },
  {
    background: '--negative',
    color: '--surface',
  },
  {
    background: '--surface',
    color: '--negative',
  },
];

const template = (context: example[]): TemplateResult => html`
  ${context.map(
    (ex: example): TemplateResult =>
      html` <tr style="background: var(${ex.background})">
        <td style="color: var(${ex.color})">
          background: var(${ex.background});
        </td>
        <td style="color: var(${ex.color})">color: var(${ex.color});</td>
      </tr>`
  )}
`;

render(template(examples), document.querySelector<HTMLElement>('#demotable')!);

render(
  template(specialty),
  document.querySelector<HTMLElement>('#demotable2')!
);

document.querySelector('#toggle-collapse-sk')?.addEventListener('click', () => {
  const collapseSk = document.querySelector<CollapseSk>('collapse-sk')!;
  collapseSk.closed = !collapseSk.closed;
});

document.querySelector('#make-toast')?.addEventListener('click', () => {
  document.querySelector<ToastSk>('toast-sk')?.show();
});

document.querySelector('#hide-toast')?.addEventListener('click', () => {
  document.querySelector<ToastSk>('toast-sk')?.hide();
});

document.querySelector('#show-error-toast')?.addEventListener('click', () => {
  errorMessage('Oh no, there was a problem!', 0);
});

document.querySelectorAll("[data-show='1']").forEach((ele) => {
  const pre = document.createElement('pre');

  const dup = ele.cloneNode(true) as Element;
  dup.removeAttribute('data-show');

  pre.innerText = dup.outerHTML.replace('=""', '');
  ele.parentElement?.insertBefore(pre, ele);
});

// Handle dynamic updates to the theme.
const primary = document.querySelector<HTMLInputElement>('#primary')!;
const secondary = document.querySelector<HTMLInputElement>('#secondary')!;

const updateTokens = () => {
  // Remove all existing style elements in head.
  $('head style').forEach((ele) => ele.remove());

  // Create a style element.
  const style = document.createElement('style');

  // Add the CSS as a string.
  style.innerHTML = gentheme(primary.value, secondary.value);

  // Add element to the head.
  $$('head')!.appendChild(style);
  $$('#generate-cmd')!.textContent =
    `//go:generate bazelisk run --config=mayberemote //infra-sk/modules/gentheme/cmd:gentheme -- ${primary.value.slice(
      1
    )} ${secondary.value.slice(1)} $\{PWD}/tokens.scss`;
};

document.querySelector('#primary')!.addEventListener('input', updateTokens);
document.querySelector('#secondary')!.addEventListener('input', updateTokens);
