import './index';
import { $, $$ } from 'common-sk/modules/dom';
import { eventPromise, noEventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { CorpusSelectorSk } from './corpus-selector-sk';
import { stringCorpora, customTypeCorpora, TestCorpus } from './test_data';

const expect = chai.expect;

describe('corpus-selector-sk', () => {
  describe('with string corpora', () => {
    const newInstance = setUpElementUnderTest<CorpusSelectorSk<string>>('corpus-selector-sk');

    let corpusSelectorSk: CorpusSelectorSk<string>;

    beforeEach(() => {
      corpusSelectorSk = newInstance();
      corpusSelectorSk.corpora = stringCorpora;
      corpusSelectorSk.selectedCorpus = 'gm';
    });

    it('shows a loading indicator when the corpora is empty', () => {
      corpusSelectorSk.corpora = [];
      expect(corpusSelectorSk.innerText).to.equal('Loading corpora details...');
    });

    it('shows the available corpora', () => {
      expect(availableCorporaOnUI(corpusSelectorSk)).to.deep.equal([
        'canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg'
      ]);
    });

    it('shows the available corpora using a custom renderer function', () => {
      corpusSelectorSk.corpusRendererFn = (corpus) => `(${corpus})`;
      expect(availableCorporaOnUI(corpusSelectorSk)).to.deep.equal([
        '(canvaskit)', '(colorImage)', '(gm)', '(image)', '(pathkit)', '(skp)', '(svg)'
      ]);
    });

    it('shows the selected corpus', () => {
      expect(selectedCorpusOnUI(corpusSelectorSk)).to.equal('gm');
    })

    it('can handle an empty selection', () => {
      corpusSelectorSk.selectedCorpus = null;
      expect(selectedCorpusOnUI(corpusSelectorSk)).to.be.undefined;
    });

    it('can change the selection programmatically', () => {
      corpusSelectorSk.selectedCorpus = 'image';
      expect(corpusSelectorSk.selectedCorpus).to.equal('image');
      expect(selectedCorpusOnUI(corpusSelectorSk)).to.equal('image');
    });

    it('does not emit "corpus-selected" when selection changes programmatically', async () => {
      const noEvent = noEventPromise('corpus-selected');
      corpusSelectorSk.selectedCorpus = 'image';
      await noEvent;
    });

    describe('clicking on a corpus', () => {
      it('changes the selection', () => {
        clickCorpus(corpusSelectorSk, 'colorImage');
        expect(corpusSelectorSk.selectedCorpus).to.equal('colorImage');
        expect(selectedCorpusOnUI(corpusSelectorSk)).to.equal('colorImage');
      });

      it('emits event "corpus-selected" with the new corpus', async () => {
        const corpusSelected = eventPromise<CustomEvent<string>>('corpus-selected');
        clickCorpus(corpusSelectorSk, 'colorImage');
        const newSelection = (await corpusSelected).detail;
        expect(newSelection).to.equal('colorImage');
      });

      it('does not emit "corpus-selected" if it\'s the current corpus', async () => {
        const noEvent = noEventPromise('corpus-selected');
        clickCorpus(corpusSelectorSk, 'gm');
        await noEvent;
      })
    });
  });

  describe('with a custom corpus object type and corpus renderer function', () => {
    const newInstance = setUpElementUnderTest<CorpusSelectorSk<TestCorpus>>('corpus-selector-sk');

    let corpusSelectorSk: CorpusSelectorSk<TestCorpus>;

    beforeEach(() => {
      corpusSelectorSk = newInstance();
      corpusSelectorSk.corpusRendererFn =
        (c: TestCorpus) => `${c.name} : ${c.untriagedCount} / ${c.negativeCount}`;
      corpusSelectorSk.corpora = customTypeCorpora;
      corpusSelectorSk.selectedCorpus = customTypeCorpora.find(corpus => corpus.name === 'gm')!;
    });

    it('handles a non-trivial custom corpus renderer function', () => {
      expect(availableCorporaOnUI(corpusSelectorSk)).to.deep.equal([
          'canvaskit : 2 / 2',
          'colorImage : 0 / 1',
          'gm : 61 / 1494',
          'image : 22 / 35',
          'pathkit : 0 / 0',
          'skp : 0 / 1',
          'svg : 19 / 21'
      ]);
    });

    it('changes the selection when clicking on a corpus', () => {
      clickCorpus(corpusSelectorSk, 'skp : 0 / 1');
      expect(corpusSelectorSk.selectedCorpus)
        .to.deep.equal(customTypeCorpora.find(corpus => corpus.name === 'skp')!);
      expect(selectedCorpusOnUI(corpusSelectorSk)).to.equal('skp : 0 / 1');
    });

    it('emits "corpus-selected" with custom corpus object when selection changes', async () => {
      const corpusSelected = eventPromise<CustomEvent<TestCorpus>>('corpus-selected');
      clickCorpus(corpusSelectorSk, 'skp : 0 / 1');
      const newSelection = (await corpusSelected).detail;
      expect(newSelection).to.deep.equal(customTypeCorpora.find(corpus => corpus.name === 'skp')!);
    });
  });
});

const availableCorporaOnUI =
  <T>(el: CorpusSelectorSk<T>) => $<HTMLLIElement>('li', el).map((li) => li.innerText);

const selectedCorpusOnUI =
  <T>(el: CorpusSelectorSk<T>) => $$<HTMLLIElement>('li.selected', el)?.innerText;

const clickCorpus =
  <T>(el: CorpusSelectorSk<T>, label: string) =>
    $<HTMLLIElement>('li').find(li => li.innerText === label)!.click();
