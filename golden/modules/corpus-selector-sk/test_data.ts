/** Simple test corpora composed of plain string objects. */
export const stringCorpora = ['canvaskit', 'colorImage', 'gm', 'image', 'pathkit', 'skp', 'svg'];

/** A custom corpus object type with various fields. */
export interface TestCorpus {
  name: string,
  ok: boolean,
  minCommitHash: string,
  untriagedCount: number,
  negativeCount: number
}

/** These examples are based on a real request against https://gold.skia.org/json/v1/trstatus. */
export const customTypeCorpora: TestCorpus[] = [{
  name: 'canvaskit',
  ok: false,
  minCommitHash: '',
  untriagedCount: 2,
  negativeCount: 2,
}, {
  name: 'colorImage',
  ok: true,
  minCommitHash: '',
  untriagedCount: 0,
  negativeCount: 1,
}, {
  name: 'gm',
  ok: false,
  minCommitHash: '',
  untriagedCount: 61,
  negativeCount: 1494,
}, {
  name: 'image',
  ok: false,
  minCommitHash: '',
  untriagedCount: 22,
  negativeCount: 35,
}, {
  name: 'pathkit',
  ok: true,
  minCommitHash: '',
  untriagedCount: 0,
  negativeCount: 0,
}, {
  name: 'skp',
  ok: true,
  minCommitHash: '',
  untriagedCount: 0,
  negativeCount: 1,
}, {
  name: 'svg',
  ok: false,
  minCommitHash: '',
  untriagedCount: 19,
  negativeCount: 21,
}];
