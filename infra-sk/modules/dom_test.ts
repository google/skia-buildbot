// Copyright 2019 Google LLC
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

import { assert } from 'chai';
import {
  $, $$, findParent, findParentSafe,
} from './dom';

const container = document.createElement('div');
document.body.appendChild(container);

beforeEach(() => {
  container.innerHTML = `
  <div id=delta class=alpha></div>
  <span class=alpha></span>
  <span class=beta></span>
  <video id=epsilon class=alpha></video>
`;
});

afterEach(() => {
  container.innerHTML = '';
});

describe('$ aka querySelectorAll', () => {
  // checks that each "array-like" thing has
  // the same things in the same indices.
  function assertEquals<T extends Element>(arr: T[], qsa: NodeListOf<T>) {
    assert.isOk(arr);
    assert.equal(arr.length, qsa.length);
    for (let i = 0; i < arr.length; i++) {
      assert.equal(arr[i], qsa[i]);
    }
  }

  it('should mimic querySelectorAll', () => {
    assertEquals($('.alpha', container), container.querySelectorAll('.alpha'));
    assertEquals(
      $('#epsilon', container),
      container.querySelectorAll('#epsilon'),
    );
    assertEquals($('span', container), container.querySelectorAll('span'));
  });

  it('should default to document', () => {
    assertEquals($('.alpha'), document.querySelectorAll('.alpha'));
    assertEquals($('#epsilon'), document.querySelectorAll('#epsilon'));
    assertEquals($('span'), document.querySelectorAll('span'));
  });

  it('should return a real array', () => {
    const arr = $('.alpha');
    assert.isTrue(Array.isArray(arr));
  });

  it('returns empty array if not found', () => {
    const arr = $('#not-found');
    assert.deepEqual([], arr);
  });
});

describe('$$ aka querySelector', () => {
  it('should mimic querySelector', () => {
    assert.equal($$('.alpha', container), container.querySelector('.alpha'));
    assert.equal(
      $$('#epsilon', container),
      container.querySelector('#epsilon'),
    );
    assert.equal($$('span', container), container.querySelector('span'));
  });

  it('should default to document', () => {
    assert.equal($$('.alpha'), document.querySelector('.alpha'));
    assert.equal($$('#epsilon'), document.querySelector('#epsilon'));
    assert.equal($$('span'), document.querySelector('span'));
  });

  it('returns a single item', () => {
    const ele = $$('.alpha');
    assert.isFalse(Array.isArray(ele));
    assert.isNotNull(ele);
    if (ele !== null) {
      assert.equal('delta', ele.id);
    }
  });
});

describe('findParent', () => {
  it('identifies the correct parent element', () => {
    // Add an HTML tree to the document.
    const div = document.createElement('div');
    div.innerHTML = `
      <div id=a>
        <p id=aa>
          <span id=aaa>span</span>
          <span id=aab>span</span>
        </p>
        <span id=ab>
          <p id=aba>para</p>
        </span>
        <div id=ac>
          <p id=aca>para</p>
        </div>
      </div>
      <div id=b>
        <p id=ba>para</p>
      </div>
      <span id=c>
        <span id=ca>
          <p id=caa>para</p>
        </span>
      </span>`;
    assert.equal(findParent($$('#a', div), 'DIV'), $$('#a', div), 'Top level');
    assert.equal(findParent($$('#a', div), 'SPAN'), null);
    assert.equal(findParent($$('#aa', div), 'DIV'), $$('#a', div));
    assert.equal(findParent($$('#aaa', div), 'DIV'), $$('#a', div));
    assert.equal(findParent($$('#aaa', div), 'P'), $$('#aa', div));
    assert.equal(findParent($$('#aab', div), 'SPAN'), $$('#aab', div));
    assert.equal(findParent($$('#ab', div), 'P'), null);
    assert.equal(findParent($$('#aba', div), 'SPAN'), $$('#ab', div));
    assert.equal(findParent($$('#ac', div), 'DIV'), $$('#ac', div));
    assert.equal(findParent($$('#aca', div), 'DIV'), $$('#ac', div));
    assert.equal(findParent($$('#ba', div), 'DIV'), $$('#b', div));
    assert.equal(findParent($$('#caa', div), 'DIV'), div);
    assert.equal(findParent($$('#ca', div), 'SPAN'), $$('#ca', div));
    assert.equal(findParent($$('#caa', div), 'SPAN'), $$('#ca', div));
  });
});

describe('findParentSafe', () => {
  it('identifies the correct parent element', () => {
    // Add an HTML tree to the document.
    const div = document.createElement('div');
    div.innerHTML = `
      <div id=a>
        <p id=aa>
          <span id=aaa>span</span>
          <span id=aab>span</span>
        </p>
        <span id=ab>
          <p id=aba>para</p>
        </span>
        <div id=ac>
          <p id=aca>para</p>
        </div>
      </div>
      <div id=b>
        <p id=ba>para</p>
      </div>
      <span id=c>
        <span id=ca>
          <p id=caa>para</p>
        </span>
      </span>`;
    assert.equal(
      findParentSafe($$('#a', div), 'div'),
      $$('#a', div),
      'Top level',
    );
    assert.equal(findParentSafe($$('#a', div), 'span'), null);
    assert.equal(findParentSafe($$('#aa', div), 'div'), $$('#a', div));
    assert.equal(findParentSafe($$('#aaa', div), 'div'), $$('#a', div));
    assert.equal(findParentSafe($$('#aaa', div), 'p'), $$('#aa', div));
    assert.equal(findParentSafe($$('#aab', div), 'span'), $$('#aab', div));
    assert.equal(findParentSafe($$('#ab', div), 'p'), null);
    assert.equal(findParentSafe($$('#aba', div), 'span'), $$('#ab', div));
    assert.equal(findParentSafe($$('#ac', div), 'div'), $$('#ac', div));
    assert.equal(findParentSafe($$('#aca', div), 'div'), $$('#ac', div));
    assert.equal(findParentSafe($$('#ba', div), 'div'), $$('#b', div));
    assert.equal(findParentSafe($$('#caa', div), 'div'), div);
    assert.equal(findParentSafe($$('#ca', div), 'span'), $$('#ca', div));
    assert.equal(findParentSafe($$('#caa', div), 'span'), $$('#ca', div));
  });
});
