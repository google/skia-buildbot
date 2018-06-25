import { $, $$ } from './dom.js'

let container = document.createElement('div');
document.body.appendChild(container);

beforeEach(function() {
  container.innerHTML = `
  <div id=delta class=alpha></div>
  <span class=alpha></span>
  <span class=beta></span>
  <video id=epsilon class=alpha></video>
`
});

afterEach(function() {
  container.innerHTML = '';
});

describe('$ aka querySelectorAll', function() {
  // checks that each "array-like" thing has
  // the same things in the same indices.
  // Useful for comparing the "not-quite-Array" that
  // querySelectorAll returns.
  function assertEquals(arr, qsa) {
    assert.isOk(arr);
    assert.equal(arr.length, qsa.length);
    for (let i = 0; i < arr.length; i++) {
      assert.equal(arr[i], qsa[i]);
    }
  }

  it('should mimic querySelectorAll', function(){
    assertEquals($('.alpha', container),
                 container.querySelectorAll('.alpha'));
    assertEquals($('#epsilon', container),
                 container.querySelectorAll('#epsilon'));
    assertEquals($('span', container),
                 container.querySelectorAll('span'));
  });

  it('should default to document', function(){
    assertEquals($('.alpha'),
                 document.querySelectorAll('.alpha'));
    assertEquals($('#epsilon'),
                 document.querySelectorAll('#epsilon'));
    assertEquals($('span'),
                 document.querySelectorAll('span'));
  });

  it('should return a real array', function(){
    let arr = $('.alpha');
    assert.isTrue(Array.isArray(arr));
  });

  it('returns empty array if not found', function(){
    let arr = $('#not-found');
    assert.deepEqual([], arr);
  });
});

describe('$$ aka querySelector', function() {
  it('should mimic querySelector', function(){
    assert.equal($$('.alpha', container),
                 container.querySelector('.alpha'));
    assert.equal($$('#epsilon', container),
                 container.querySelector('#epsilon'));
    assert.equal($$('span', container),
                 container.querySelector('span'));
  });

  it('should default to document', function(){
    assert.equal($$('.alpha'),
                 document.querySelector('.alpha'));
    assert.equal($$('#epsilon'),
                 document.querySelector('#epsilon'));
    assert.equal($$('span'),
                 document.querySelector('span'));
  });

  it('returns a single item', function(){
    let ele = $$('.alpha');
    assert.isFalse(Array.isArray(ele));
    assert.equal('delta', ele.id);
  });

});
