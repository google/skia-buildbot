export const examplePageData = {
  alpha_test: {
    aaaaaaaaaaaaaaaaaaaaaaaaaaa: 'positive',
    bbbbbbbbbbbbbbbbbbbbbbbbbbb: 'negative',
  },

  beta_test: {
    ccccccccccccccccccccccccccc: 'positive',
  },
};

export const expectedPageData = '{"testDigestStatus":{"alpha_test":{"aaaaaaaaaaaaaaaaaaaaaaaaaaa":"positive","bbbbbbbbbbbbbbbbbbbbbbbbbbb":"negative"},"beta_test":{"ccccccccccccccccccccccccccc":"positive"}},"changelist_id":"","crs":""}';

export const exampleAllData = {
  alpha_test: {
    aaaaaaaaaaaaaaaaaaaaaaaaaaa: 'positive',
    bbbbbbbbbbbbbbbbbbbbbbbbbbb: 'negative',
    ddddddddddddddddddddddddddd: 'positive',
  },

  beta_test: {
    ccccccccccccccccccccccccccc: 'positive',
    ddddddddddddddddddddddddddd: 'negative',
  },

  gamma_test: {
    eeeeeeeeeeeeeeeeeeeeeeeeeee: '', // pretend this has no closest reference image.
  },
};

export const expectedAllData = '{"testDigestStatus":{"alpha_test":{"aaaaaaaaaaaaaaaaaaaaaaaaaaa":"positive","bbbbbbbbbbbbbbbbbbbbbbbbbbb":"negative","ddddddddddddddddddddddddddd":"positive"},"beta_test":{"ccccccccccccccccccccccccccc":"positive","ddddddddddddddddddddddddddd":"negative"},"gamma_test":{"eeeeeeeeeeeeeeeeeeeeeeeeeee":""}},"changelist_id":"someCL","crs":"gerrit"}';
