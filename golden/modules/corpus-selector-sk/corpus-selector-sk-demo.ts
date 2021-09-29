import './index';
import { $$ } from 'common-sk/modules/dom';
import { CorpusSelectorSk } from './corpus-selector-sk';
import { TestCorpus, customTypeCorpora, stringCorpora } from './test_data';

const handleCorpusSelected = (corpusName: string) => {
  const log = $$<HTMLTextAreaElement>('#event-log')!;
  log.value = `${corpusName.padEnd(15) + new Date()}\n${log.value}`;
};

// Basic example using the default corpus render function and simple string corpora.
const basicExample = new CorpusSelectorSk();
basicExample.corpora = stringCorpora;
basicExample.selectedCorpus = 'gm';
basicExample.addEventListener(
  'corpus-selected', (e: Event) => handleCorpusSelected((e as CustomEvent<string>).detail),
);
$$('#default')!.appendChild(basicExample);

// Example using a more interesting corpus type and a custom corpus renderer function.
const withCustomRendererFn = new CorpusSelectorSk<TestCorpus>();
withCustomRendererFn.corpora = customTypeCorpora;
withCustomRendererFn.selectedCorpus = customTypeCorpora.find((c) => c.name === 'gm')!;
withCustomRendererFn.corpusRendererFn = (corpus) => `${corpus.name} : ${corpus.untriagedCount} / ${corpus.negativeCount}`;
withCustomRendererFn.addEventListener(
  'corpus-selected', (e) => handleCorpusSelected((e as CustomEvent<TestCorpus>).detail.name),
);
$$('#custom-fn')!.appendChild(withCustomRendererFn);

// Example using a custom corpus renderer function that produces long corpus names.
const withLongCorpusNames = new CorpusSelectorSk();
withLongCorpusNames.corpora = stringCorpora;
withLongCorpusNames.selectedCorpus = customTypeCorpora.find((c) => c.name === 'gm')!;
withLongCorpusNames.corpusRendererFn = (corpus) => `${corpus} : yadda yadda yadda yadda yadda`;
withLongCorpusNames.addEventListener(
  'corpus-selected', (e) => handleCorpusSelected((e as CustomEvent<string>).detail),
);
$$('#custom-fn-long-corpus')!.appendChild(withLongCorpusNames);
