
export function displaySilence(silence) {
  let ret = [];
  for (let key in silence.param_set) {
    if (key.startsWith('__')) {
      continue
    }
    ret.push(`${key} - ${silence.param_set[key].join(', ')}`);
  }
  let s = ret.join(' ');
  if (s.length > 33) {
    s = s.slice(0, 30) + '...';
  }
  if (s.length == 0) {
    s = '(*)';
  }
  return s;
}

export function abbr(ele) {
  let s = ele.params['abbr'];
  if (s) {
    return ` - ${s}`;
  } else {
    return ``
  }
}

export function linkify(s) {
  return unsafeHTML(s.replace(linkRe, '<a href="$&">$&</a>'));
}

export function notes(ele) {
  if (!ele._state.notes) {
    return [];
  }
  return ele._state.notes.map((note, index) => html`<section class=note>
  <p>${linkify(note.text)}</p>
  <div class=meta>
    <span class=author>${note.author}</span>
    <span class=date>${diffDate(note.ts*1000)}</span>
    <delete-icon-sk title='Delete comment.' on-click=${(e) => ele._deleteNote(e, index)}></delete-icon-sk>
  </div>
</section>`);
}
