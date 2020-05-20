// Functions used by more than one element.
import { diffDate } from 'common-sk/modules/human';
import { unsafeHTML } from 'lit-html/directives/unsafe-html';
import { html } from 'lit-html';

const linkRe = /(http[s]?:\/\/[^\s]*)/gm;

/**
 * Formats the text for a silence header.
 *
 * silence - The silence being displayed.
 */
export function displaySilence(silence) {
  const ret = [];
  for (const key in silence.param_set) {
    if (key.startsWith('__')) {
      continue;
    }
    ret.push(`${silence.param_set[key].join(', ')}`);
  }
  let s = ret.join(' ');
  if (s.length > 33) {
    s = `${s.slice(0, 30)}...`;
  }
  if (!s.length) {
    s = '(*)';
  }
  return s;
}

/**
 * Returns the params.abbr to be appended to a string, if present.
 */
export function abbr(ele) {
  const s = ele.params.abbr;
  if (s) {
    return ` - ${s}`;
  }
  return '';
}

/**
 * Convert all URLs in a string into links in a lit-html TemplateResult.
 */
export function linkify(s) {
  return unsafeHTML(s.replace(linkRe, '<a href="$&" rel=noopener target=_blank>$&</a>'));
}

/**
 * Templates notes to be displayed.
 */
export function notes(ele) {
  if (!ele._state.notes) {
    return [];
  }
  return ele._state.notes.map((note, index) => html`<section class=note>
  <p>${linkify(note.text)}</p>
  <div class=meta>
    <span class=author>${note.author}</span>
    <span class=date>${diffDate(note.ts * 1000)}</span>
    <delete-icon-sk title='Delete comment.' @click=${(e) => ele._deleteNote(e, index)}></delete-icon-sk>
  </div>
</section>`);
}

const TIME_DELTAS = [
  { units: 'w', delta: 7 * 24 * 60 * 60 },
  { units: 'd', delta: 24 * 60 * 60 },
  { units: 'h', delta: 60 * 60 },
  { units: 'm', delta: 60 },
  { units: 's', delta: 1 },
];

/**
 * Returns the parsed duration in seconds.
 *
 * @param {string} d - The duration, e.g. "2h" or "4d".
 * @returns {number} The duration in seconds.
 *
 * TODO(jcgregorio) Move into common-sk/modules/human.js with tests.
 */
export function parseDuration(d) {
  const units = d.slice(-1);
  const scalar = +d.slice(0, -1);
  for (let i = 0; i < TIME_DELTAS.length; i++) {
    const o = TIME_DELTAS[i];
    if (o.units === units) {
      return o.delta * scalar;
    }
  }
  return 0;
}

export function expiresIn(silence) {
  if (silence.active) {
    return diffDate((silence.created + parseDuration(silence.duration)) * 1000);
  }
  return '';
}
