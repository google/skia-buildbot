import './index';
import { $$ } from 'common-sk/modules/dom';

(function() {
  // A reusable HTML element in which we create our element under test.
  const container = document.createElement('div');
  document.body.appendChild(container);

  afterEach(() => {
    container.innerHTML = '';
  });

  describe('named-edit-sk', () => {
    it('emits correct event on hash change', () => window.customElements.whenDefined('named-edit-sk').then(() => {
      container.innerHTML = '<named-edit-sk></named-edit-sk>';
      const ele = container.firstElementChild;
      ele.state = {
        name: 'Octopus_Generator_Animated',
        hash: 'ad161cfe21bb38bcec264bbacecbe93a',
        status: 'OK',
      };

      assert.isNotNull($$('h2', ele));
      assert.isNotNull($$('#name', ele));
      assert.isNotNull($$('#hash', ele));

      $$('#hash', ele).value = '123';
      let hash = '';
      ele.addEventListener('named-edit-complete', (e) => {
        hash = e.detail.hash;
      });
      $$('#ok', ele).click();
      return Promise.resolve().then(() => {
        assert.equal('123', hash);
        assert.equal($$('dialog', ele).open, false);
      });
    }));
  });

  describe('named-edit-sk', () => {
    it('emits correct event on name change', () => window.customElements.whenDefined('named-edit-sk').then(() => {
      container.innerHTML = '<named-edit-sk></named-edit-sk>';
      const ele = container.firstElementChild;
      ele.state = {
        name: 'Octopus_Generator_Animated',
        hash: 'ad161cfe21bb38bcec264bbacecbe93a',
        status: 'OK',
      };

      $$('#name', ele).value = 'some new name';
      let detail = {};
      ele.addEventListener('named-edit-complete', (e) => {
        detail = e.detail;
      });
      $$('#ok', ele).click();
      return Promise.resolve().then(() => {
        assert.equal($$('dialog', ele).open, false);
        assert.equal('ad161cfe21bb38bcec264bbacecbe93a', detail.hash);
        assert.equal('Octopus_Generator_Animated', detail.name);
        assert.equal('some new name', detail.new_name);
      });
    }));
  });
}());
