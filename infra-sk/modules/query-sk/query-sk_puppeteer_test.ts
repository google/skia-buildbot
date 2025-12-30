import { expect } from 'chai';
import { loadCachedTestBed, takeScreenshot, TestBed } from '../../../puppeteer-tests/util';
import { ElementHandle } from 'puppeteer';
import { QuerySkPO } from './query-sk_po';

describe('query-sk', () => {
  let testBed: TestBed;
  before(async () => {
    testBed = await loadCachedTestBed();
  });

  let querySk: ElementHandle;
  let querySkPO: QuerySkPO;

  beforeEach(async () => {
    await testBed.page.goto(testBed.baseUrl);
    await testBed.page.setViewport({ width: 600, height: 1400 });
    querySk = (await testBed.page.$('query-sk'))!;
    querySkPO = new QuerySkPO(querySk);
  });

  describe('screenshots', () => {
    it('shows the default view', async () => {
      console.log('Running screenshot test');
      await takeScreenshot(testBed.page, 'infra-sk', 'query-sk');
    });
  });

  describe('user interactions', () => {
    it('sets the current query and updates UI', async () => {
      console.log('Running "sets the current query and updates UI" test');
      await querySk.evaluate((el) => {
        (el as any).current_query = 'config=8888&type=GPU';
      });
      await querySkPO.clickKey('config');
      let selected = await querySkPO.getSelectedValues();
      expect(selected).to.deep.equal(['8888']);

      await querySkPO.clickKey('type');
      selected = await querySkPO.getSelectedValues();
      expect(selected).to.deep.equal(['GPU']);
      console.log('"sets the current query and updates UI" test passed');
    });

    it('selects key and values and emits event', async () => {
      console.log('Running "selects key and values and emits event" test');
      // Listen for the query-change event.
      const eventPromise = testBed.page.evaluate(
        () =>
          new Promise((resolve) => {
            document.querySelector('query-sk')!.addEventListener(
              'query-change',
              (e) => {
                resolve((e as CustomEvent).detail.q);
              },
              { once: true }
            );
          })
      );

      await querySkPO.clickKey('type');
      await querySkPO.clickValue('CPU');

      const eventDetail = await eventPromise;
      expect(eventDetail).to.equal('type=CPU');

      const query = await querySk.evaluate((el) => (el as any).current_query);
      expect(query).to.equal('type=CPU');
      console.log('"selects key and values and emits event" test passed');
    });

    it('emits query-change-delayed event', async () => {
      console.log('Running "emits query-change-delayed event" test');
      const eventPromise = testBed.page.evaluate(
        () =>
          new Promise((resolve) => {
            document.querySelector('query-sk')!.addEventListener(
              'query-change-delayed',
              (e) => {
                resolve((e as CustomEvent).detail.q);
              },
              { once: true }
            );
          })
      );

      await querySkPO.clickKey('type');
      await querySkPO.clickValue('CPU');

      const eventDetail = await eventPromise;
      expect(eventDetail).to.equal('type=CPU');
      console.log('"emits query-change-delayed event" test passed');
    });

    it('clears selections', async () => {
      console.log('Running "clears selections" test');
      await querySk.evaluate((el) => {
        (el as any).current_query = 'config=8888&type=GPU';
      });

      await querySkPO.clickClearSelections();

      const query = await querySk.evaluate((el) => (el as any).current_query);
      expect(query).to.equal('');
      console.log('"clears selections" test passed');
    });

    it('filters keys and values', async () => {
      console.log('Running "filters keys and values" test');
      await querySkPO.setFilter('one');

      const keys = await querySkPO.getKeys();
      expect(keys).to.not.contain('config');
      expect(keys).to.contain('test');

      await querySkPO.clickKey('test');
      const values = await querySkPO.queryValuesSkPO.getOptions();
      expect(values).to.have.length(5);
      expect(values).to.not.contain('DeferredSurfaceCopy_discardable');

      await querySkPO.clickClearFilter();
      const allValues = await querySkPO.queryValuesSkPO.getOptions();
      expect(allValues).to.have.length(5);
    });

    it('selects an inverted query', async () => {
      await querySkPO.clickKey('type');
      await querySkPO.queryValuesSkPO.clickInvertCheckbox();
      await querySkPO.clickValue('CPU');
      const query = await querySk.evaluate((el) => (el as any).current_query);
      expect(query).to.equal('type=!CPU');
    });

    it('selects a regex query', async () => {
      await querySkPO.clickKey('test');
      await querySkPO.queryValuesSkPO.clickRegexCheckbox();
      await querySkPO.queryValuesSkPO.setRegexValue('^GL.*Bench_one_.$');
      const query = await querySk.evaluate((el) => (el as any).current_query);
      expect(query).to.equal('test=~%5EGL.*Bench_one_.%24');
    });

    it('rationalizes the query', async () => {
      await querySk.evaluate((el) => {
        (el as any).current_query = 'config=bogus&invalid=key&type=GPU';
      });
      const query = await querySk.evaluate((el) => (el as any).current_query);
      expect(query).to.equal('type=GPU');
    });
  });

  describe('attribute functionality', () => {
    it('should only show values for values_only element', async () => {
      const valuesOnlyQuerySk = (await testBed.page.$('#valuesOnly'))!;
      const valuesOnlyPO = new QuerySkPO(valuesOnlyQuerySk);

      // In the demo, the "type" key is pre-selected for this element.
      await valuesOnlyPO.getKeys();

      const values = await valuesOnlyPO.queryValuesSkPO.getOptions();
      expect(values).to.deep.equal(['CPU', 'GPU']);

      const selected = await valuesOnlyPO.getSelectedValues();
      expect(selected).to.empty;
    });

    it('hides invert and regex', async () => {
      await querySk.evaluate((el) => {
        (el as any).hide_invert = true;
        (el as any).hide_regex = true;
      });
      await querySkPO.clickKey('type');
      expect(await querySkPO.queryValuesSkPO.isInvertCheckboxHidden()).to.be.true;
      expect(await querySkPO.queryValuesSkPO.isRegexCheckboxHidden()).to.be.true;
    });
  });

  describe('property functionality', () => {
    it('orders keys by key_order', async () => {
      console.log('Running "orders keys by key_order" test');
      const keys = await querySkPO.getKeys();
      // from query-sk-demo.ts: ele.key_order = ['test', 'units'];
      expect(keys.slice(0, 2)).to.deep.equal(['test', 'units']);
      // The rest of the keys should be alphabetical
      expect(keys.slice(2)).to.deep.equal(['config', 'type']);
      console.log('"orders keys by key_order" test passed');
    });
  });

  describe('public methods', () => {
    it('selects a key via selectKey', async () => {
      console.log('Running "selects a key via selectKey" test');
      await querySk.evaluate((el) => {
        (el as any).selectKey('config');
      });
      const selectedKey = await querySkPO.getSelectedKey();
      expect(selectedKey).to.equal('config');

      const values = await querySkPO.queryValuesSkPO.getOptions();
      expect(values).to.deep.equal(['565', '8888']);
      console.log('"selects a key via selectKey" test passed');
    });

    it('removes a key-value pair via removeKeyValue', async () => {
      console.log('Running "removes a key-value pair via removeKeyValue" test');
      await querySk.evaluate((el) => {
        (el as any).removeKeyValue('test', 'GLInstancedArraysBench_one_4');
      });

      await querySkPO.clickKey('test');
      const values = await querySkPO.queryValuesSkPO.getOptions();
      expect(values).to.not.contain('GLInstancedArraysBench_one_4');
      expect(values).to.have.length(14);
      console.log('"removes a key-value pair via removeKeyValue" test passed');
    });
  });
});
