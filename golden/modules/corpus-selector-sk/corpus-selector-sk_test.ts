import './index';
import { expect } from 'chai';
import { eventPromise, noEventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CorpusSelectorSk } from './corpus-selector-sk';
import { CorpusSelectorSkPO } from './corpus-selector-sk_po';
import { stringCorpora, customTypeCorpora, TestCorpus } from './test_data';

describe('corpus-selector-sk', () => {
  describe('with string corpora', () => {
    const newInstance = setUpElementUnderTest<CorpusSelectorSk<string>>('corpus-selector-sk');

    let corpusSelectorSk: CorpusSelectorSk<string>;
    let corpusSelectorSkPO: CorpusSelectorSkPO;

    beforeEach(() => {
      corpusSelectorSk = newInstance();
      corpusSelectorSk.corpora = stringCorpora;
      corpusSelectorSk.selectedCorpus = 'gm';

      corpusSelectorSkPO = new CorpusSelectorSkPO(corpusSelectorSk);
    });

    it('shows a loading indicator when the corpora is empty', async () => {
      corpusSelectorSk.corpora = [];
      expect(await corpusSelectorSkPO.isLoadingMessageVisible()).to.be.true;
    });

    it('shows the available corpora', async () => {
      expect(await corpusSelectorSkPO.getCorpora()).to.deep.equal([
        'canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg',
      ]);
    });

    it('shows the available corpora using a custom renderer function', async () => {
      corpusSelectorSk.corpusRendererFn = (corpus) => `(${corpus})`;
      expect(await corpusSelectorSkPO.getCorpora()).to.deep.equal([
        '(canvaskit)', '(colorImage)', '(gm)', '(image)', '(pathkit)', '(skp)', '(svg)',
      ]);
    });

    it('shows the selected corpus', async () => {
      expect(await corpusSelectorSkPO.getSelectedCorpus()).to.equal('gm');
    });

    it('can handle an empty selection', async () => {
      corpusSelectorSk.selectedCorpus = null;
      expect(await corpusSelectorSkPO.getSelectedCorpus()).to.be.null;
    });

    it('can change the selection programmatically', async () => {
      corpusSelectorSk.selectedCorpus = 'image';
      expect(corpusSelectorSk.selectedCorpus).to.equal('image');
      expect(await corpusSelectorSkPO.getSelectedCorpus()).to.equal('image');
    });

    it('does not emit "corpus-selected" when selection changes programmatically', async () => {
      const noEvent = noEventPromise('corpus-selected');
      corpusSelectorSk.selectedCorpus = 'image';
      await noEvent;
    });

    describe('clicking on a corpus', () => {
      it('changes the selection', async () => {
        await corpusSelectorSkPO.clickCorpus('colorImage');
        expect(corpusSelectorSk.selectedCorpus).to.equal('colorImage');
        expect(await corpusSelectorSkPO.getSelectedCorpus()).to.equal('colorImage');
      });

      it('emits event "corpus-selected" with the new corpus', async () => {
        const corpusSelected = eventPromise<CustomEvent<string>>('corpus-selected');
        await corpusSelectorSkPO.clickCorpus('colorImage');
        const newSelection = (await corpusSelected).detail;
        expect(newSelection).to.equal('colorImage');
      });

      it('does not emit "corpus-selected" if it\'s the current corpus', async () => {
        const noEvent = noEventPromise('corpus-selected');
        await corpusSelectorSkPO.clickCorpus('gm');
        await noEvent;
      });
    });
  });

  describe('with a custom corpus object type and corpus renderer function', () => {
    const newInstance = setUpElementUnderTest<CorpusSelectorSk<TestCorpus>>('corpus-selector-sk');

    let corpusSelectorSk: CorpusSelectorSk<TestCorpus>;
    let corpusSelectorSkPO: CorpusSelectorSkPO;

    beforeEach(() => {
      corpusSelectorSk = newInstance();
      corpusSelectorSk.corpusRendererFn = (c: TestCorpus) => `${c.name} : ${c.untriagedCount} / ${c.negativeCount}`;
      corpusSelectorSk.corpora = customTypeCorpora;
      corpusSelectorSk.selectedCorpus = customTypeCorpora.find((corpus) => corpus.name === 'gm')!;

      corpusSelectorSkPO = new CorpusSelectorSkPO(corpusSelectorSk);
    });

    it('handles a non-trivial custom corpus renderer function', async () => {
      expect(await corpusSelectorSkPO.getCorpora()).to.deep.equal([
        'canvaskit : 2 / 2',
        'colorImage : 0 / 1',
        'gm : 61 / 1494',
        'image : 22 / 35',
        'pathkit : 0 / 0',
        'skp : 0 / 1',
        'svg : 19 / 21',
      ]);
    });

    it('changes the selection when clicking on a corpus', async () => {
      await corpusSelectorSkPO.clickCorpus('skp : 0 / 1');
      expect(corpusSelectorSk.selectedCorpus)
        .to.deep.equal(customTypeCorpora.find((corpus) => corpus.name === 'skp')!);
      expect(await corpusSelectorSkPO.getSelectedCorpus()).to.equal('skp : 0 / 1');
    });

    it('emits "corpus-selected" with custom corpus object when selection changes', async () => {
      const corpusSelected = eventPromise<CustomEvent<TestCorpus>>('corpus-selected');
      await corpusSelectorSkPO.clickCorpus('skp : 0 / 1');
      const newSelection = (await corpusSelected).detail;
      expect(newSelection).to.deep.equal(customTypeCorpora.find((corpus) => corpus.name === 'skp')!);
    });
  });
});
