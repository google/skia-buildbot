import './index';
import { $, $$ } from 'common-sk/modules/dom';
import { eventPromise, setUpElementUnderTest } from '../../../infra-sk/modules/test_util';
import { exampleCorpora } from './test_data';

describe('corpus-selector-sk', () => {
  const newInstance = setUpElementUnderTest('corpus-selector-sk');

  const newCorpusSelectorSk = (opts = {}) => newInstance((ele) => {
    const { corpusRendererFn, selectedCorpus } = opts;
    if (corpusRendererFn) {
      ele.corpusRendererFn = corpusRendererFn;
    }
    if (selectedCorpus) {
      ele.selectedCorpus = selectedCorpus;
    }
    ele.corpora = exampleCorpora;
  });

  it('shows loading indicator', () => {
    const corpusSelectorSk = newInstance();

    expect(corpusSelectorSk.innerText).to.equal('Loading corpora details...');
  });

  it('renders corpora with unspecified default corpus', async () => {
    const corpusSelectorSk = newCorpusSelectorSk();

    expect(corporaLiText(corpusSelectorSk)).to.deep.equal(
      ['canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg'],
    );
    expect(corpusSelectorSk.selectedCorpus).to.be.undefined;
    expect(selectedCorpusLiText(corpusSelectorSk)).to.be.null;
  });

  it('renders corpora with default corpus', async () => {
    const corpusSelectorSk = newCorpusSelectorSk({ selectedCorpus: 'gm' });

    expect(corporaLiText(corpusSelectorSk)).to.deep.equal(
      ['canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg'],
    );
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');
  });

  it('renders corpora with custom function', async () => {
    const corpusSelectorSk = newCorpusSelectorSk({
      corpusRendererFn:
          (c) => `${c.name} : ${c.untriagedCount} / ${c.negativeCount}`,
    });

    expect(corporaLiText(corpusSelectorSk)).to.deep.equal([
      'canvaskit : 2 / 2',
      'colorImage : 0 / 1',
      'gm : 61 / 1494',
      'image : 22 / 35',
      'pathkit : 0 / 0',
      'skp : 0 / 1',
      'svg : 19 / 21']);
  });

  it('selects corpus and emits "corpus_selected" event when clicked', async () => {
    const corpusSelectorSk = newCorpusSelectorSk({ selectedCorpus: 'gm' });

    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');

    // Click on 'svg' corpus.
    const corpusSelected = eventPromise('corpus_selected');
    $$('li[title="svg"]', corpusSelectorSk).click();
    const ev = await corpusSelected;

    // Assert that selected corpus changed.
    expect(ev.detail.corpus).to.equal('svg');
    expect(corpusSelectorSk.selectedCorpus).to.equal('svg');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('svg');
  });

  it('can set the selected corpus programmatically', async () => {
    const corpusSelectorSk = newCorpusSelectorSk({ selectedCorpus: 'gm' });

    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');

    // Select corpus 'svg' programmatically.
    corpusSelectorSk.selectedCorpus = 'svg';

    // Assert that selected corpus changed.
    expect(corpusSelectorSk.selectedCorpus).to.equal('svg');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('svg');
  });

  it('does not trigger corpus change event if selected corpus is clicked', async () => {
    const corpusSelectorSk = newCorpusSelectorSk({ selectedCorpus: 'gm' });

    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');

    // Click on 'gm' corpus.
    corpusSelectorSk.dispatchEvent = sinon.fake();
    $$('li[title="gm"]', corpusSelectorSk).click();

    // Assert that selected corpus didn't change and that no event was emitted.
    expect(corpusSelectorSk.dispatchEvent.callCount).to.equal(0);
    expect(corpusSelectorSk.selectedCorpus).to.equal('gm');
    expect(selectedCorpusLiText(corpusSelectorSk)).to.equal('gm');
  });
});

const corporaLiText = (el) => $('li', el).map((li) => li.innerText);

const selectedCorpusLiText = (el) => {
  const li = $$('li.selected', el);
  return li ? li.innerText : null;
};
