// Copyright 2018 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import './index';
import '../tabs-panel-sk/index';
import { assert } from 'chai';
import { TabsSk } from './tabs-sk';

describe('tabs-sk', () => {
  let container: HTMLDivElement;

  before(async () => {
    await customElements.whenDefined('tabs-sk');
    await customElements.whenDefined('tabs-panel-sk');
  });

  beforeEach(() => {
    container = document.createElement('div');
    document.body.appendChild(container);
  });

  afterEach(() => {
    document.body.removeChild(container);
  });

  it('has selected attribute', () => {
    container.innerHTML = `
        <tabs-sk>
          <button>One</button>
          <button>Two</button>
        </tabs-sk>
        <tabs-panel-sk>
          <section id=panel1></section>
          <section id=panel2></section>
        </tabs-panel-sk>
        `;
    const s = container.firstElementChild as TabsSk;
    assert.equal('0', s.getAttribute('selected'));
    s.setAttribute('selected', '1');
    assert.equal('1', s.getAttribute('selected'));
    const panel1 = container.querySelector('#panel1')!;
    const panel2 = container.querySelector('#panel2')!;
    assert.isNotNull(panel2);
    assert.isFalse(panel1.classList.contains('selected'));
    assert.isTrue(panel2.classList.contains('selected'));
  }); // has selected attribute

  it('has selected property', () => {
    container.innerHTML = `
        <tabs-sk>
          <button>One</button>
          <button>Two</button>
        </tabs-sk>
        <tabs-panel-sk>
          <section id=panel1></section>
          <section id=panel2></section>
        </tabs-panel-sk>
        `;
    const s = container.firstElementChild as TabsSk;
    assert.equal(0, s.selected);
    s.selected = 1;
    assert.equal(1, s.selected);
    const panel1 = container.querySelector('#panel1')!;
    const panel2 = container.querySelector('#panel2')!;
    assert.isNotNull(panel2);
    assert.isFalse(panel1.classList.contains('selected'));
    assert.isTrue(panel2.classList.contains('selected'));
  }); // has selected property
});
