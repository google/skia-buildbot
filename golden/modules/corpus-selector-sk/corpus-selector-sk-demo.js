import './index';
import { $$ } from 'common-sk/modules/dom';
import { exampleCorpora } from './test_data';

const handleCorpusSelected = (e) => {
  const corpus = e.detail.corpus;
  const log = $$('#event-log');
  log.value = `${corpus.padEnd(15) + new Date()}\n${log.value}`;
};

// Default corpus renderer function.
let ele = document.createElement('corpus-selector-sk');
ele.corpora = exampleCorpora;
ele.selectedCorpus = 'gm';
ele.addEventListener('corpus-selected', handleCorpusSelected);
$$('#default').appendChild(ele);

// Custom corpus renderer function.
ele = document.createElement('corpus-selector-sk');
ele.corpora = exampleCorpora;
ele.selectedCorpus = 'gm';
ele.corpusRendererFn = (corpus) => `${corpus.name} : ${corpus.untriagedCount} / ${corpus.negativeCount}`;
ele.addEventListener('corpus-selected', handleCorpusSelected);
$$('#custom-fn').appendChild(ele);

// Custom corpus renderer function (long).
ele = document.createElement('corpus-selector-sk');
ele.corpora = exampleCorpora;
ele.selectedCorpus = 'gm';
ele.corpusRendererFn = (corpus) => `${corpus.name} : yadda yadda yadda yadda yadda`;
ele.addEventListener('corpus-selected', handleCorpusSelected);
$$('#custom-fn-long-corpus').appendChild(ele);
