import './index';
import { $$ } from 'common-sk/modules/dom';
import { exampleCorpora } from './test_data';

const handleCorpusSelected = (e) => {
  const corpus = e.detail.corpus;
  const log = $$('#event-log');
  log.value = `${corpus.padEnd(15) + new Date()}\n${log.value}`;
};

// Default corpus renderer function.
const ele = document.createElement('corpus-selector-sk');
ele.corpora = exampleCorpora;
ele.selectedCorpus = 'gm';
ele.addEventListener('corpus-selected', handleCorpusSelected);
$$('#default').appendChild(ele);

// Custom corpus renderer function.
const eleCustom = document.createElement('corpus-selector-sk');
eleCustom.corpora = exampleCorpora;
eleCustom.selectedCorpus = 'gm';
eleCustom.corpusRendererFn = (corpus) => `${corpus.name} : ${corpus.untriagedCount} / ${corpus.negativeCount}`;
eleCustom.addEventListener('corpus-selected', handleCorpusSelected);
$$('#custom-fn').appendChild(eleCustom);

// Custom corpus renderer function (long).
const eleLong = document.createElement('corpus-selector-sk');
eleLong.corpora = exampleCorpora;
eleLong.selectedCorpus = 'gm';
eleLong.corpusRendererFn = (corpus) => `${corpus.name} : yadda yadda yadda yadda yadda`;
eleLong.addEventListener('corpus-selected', handleCorpusSelected);
$$('#custom-fn-long-corpus').appendChild(eleLong);
