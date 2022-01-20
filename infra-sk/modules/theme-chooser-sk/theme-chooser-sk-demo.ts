import './theme-chooser-sk-demo.scss';
import { render, html, TemplateResult } from 'lit-html';
import { $$ } from 'common-sk/modules/dom';
import { DARKMODE_LOCALSTORAGE_KEY } from './theme-chooser-sk';
import 'elements-sk/icon/menu-icon-sk';

// Force the element to use the default mode set in the elements attribute.
window.localStorage.removeItem(DARKMODE_LOCALSTORAGE_KEY);

// eslint-disable-next-line import/first
import './theme-chooser-sk';

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
    background: '--on-primary',
    color: '--primary',
  },
  {
    background: '--primary-variant',
    color: '--on-primary',
  },
  {
    background: '--on-primary',
    color: '--primary-variant',
  },
  {
    background: '--secondary',
    color: '--on-secondary',
  },
  {
    background: '--on-secondary',
    color: '--secondary',
  },
  {
    background: '--primary-highlight',
    color: '--on-highlight',
  },
  {
    background: '--secondary-highlight',
    color: '--on-highlight',
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
    ${context.map((ex: example): TemplateResult => html`
    <tr style="background: var(${ex.background})">
      <td style="color: var(${ex.color})">background: var(${ex.background});</td>
      <td style="color: var(${ex.color})">color: var(${ex.color});</td>
    </tr>`)}
`;

render(template(examples), $$('#demotable')!);
