// The trace below is based on a subset of a real trace found in Skia Gold.
// This is what the dots diagram should look like (trace length = 20):
//
//   +--------------------+
//   |**765334433332211100|
//   |   11-1-1100--0000  |
//   |      22221111110000|
//   +--------------------+
//
// Where the numbers represent different colors, and a star (*) represents the
// special color used for unique digests in excess of MAX_UNIQUE_DIGESTS.
//
// Additionally, The numbers above correspond to traces.traces[i].data[j].s.

export const traces = {
  "tileSize": 20,
  "traces": [{
    "data": [{
      "x": 0,
      "y": 0,
      // Note: the backend tops out at s=8, which equals MAX_UNIQUE_DIGESTS.
      "s": 9
    }, {
      "x": 1,
      "y": 0,
      "s": 8
    }, {
      "x": 2,
      "y": 0,
      "s": 7
    }, {
      "x": 3,
      "y": 0,
      "s": 6
    }, {
      "x": 4,
      "y": 0,
      "s": 5
    }, {
      "x": 5,
      "y": 0,
      "s": 3
    }, {
      "x": 6,
      "y": 0,
      "s": 3
    }, {
      "x": 7,
      "y": 0,
      "s": 4
    }, {
      "x": 8,
      "y": 0,
      "s": 4
    }, {
      "x": 9,
      "y": 0,
      "s": 3
    }, {
      "x": 10,
      "y": 0,
      "s": 3
    }, {
      "x": 11,
      "y": 0,
      "s": 3
    }, {
      "x": 12,
      "y": 0,
      "s": 3
    }, {
      "x": 13,
      "y": 0,
      "s": 2
    }, {
      "x": 14,
      "y": 0,
      "s": 2
    }, {
      "x": 15,
      "y": 0,
      "s": 1
    }, {
      "x": 16,
      "y": 0,
      "s": 1
    }, {
      "x": 17,
      "y": 0,
      "s": 1
    }, {
      "x": 18,
      "y": 0,
      "s": 0
    }, {
      "x": 19,
      "y": 0,
      "s": 0
    }],
    "label": ",alpha=first-trace,beta=hello,gamma=world,",
    "params": {
      "alpha": "first-trace",
      "beta": "hello",
      "gamma": "world",
    }
  }, {
    "data": [{
      "x": 3,
      "y": 1,
      "s": 1
    }, {
      "x": 4,
      "y": 1,
      "s": 1
    }, {
      "x": 6,
      "y": 1,
      "s": 1
    }, {
      "x": 8,
      "y": 1,
      "s": 1
    }, {
      "x": 9,
      "y": 1,
      "s": 1
    }, {
      "x": 10,
      "y": 1,
      "s": 0
    }, {
      "x": 11,
      "y": 1,
      "s": 0
    }, {
      "x": 14,
      "y": 1,
      "s": 0
    }, {
      "x": 15,
      "y": 1,
      "s": 0
    }, {
      "x": 16,
      "y": 1,
      "s": 0
    }, {
      "x": 17,
      "y": 1,
      "s": 0
    }],
    "label": ",alpha=second-trace,beta=foo,gamma=bar,",
    "params": {
      "alpha": "second-trace",
      "beta": "foo",
      "gamma": "bar",
    }
  }, {
    "data": [{
      "x": 6,
      "y": 2,
      "s": 2
    }, {
      "x": 7,
      "y": 2,
      "s": 2
    }, {
      "x": 8,
      "y": 2,
      "s": 2
    }, {
      "x": 9,
      "y": 2,
      "s": 2
    }, {
      "x": 10,
      "y": 2,
      "s": 1
    }, {
      "x": 11,
      "y": 2,
      "s": 1
    }, {
      "x": 12,
      "y": 2,
      "s": 1
    }, {
      "x": 13,
      "y": 2,
      "s": 1
    }, {
      "x": 14,
      "y": 2,
      "s": 1
    }, {
      "x": 15,
      "y": 2,
      "s": 1
    }, {
      "x": 16,
      "y": 2,
      "s": 0
    }, {
      "x": 17,
      "y": 2,
      "s": 0
    }, {
      "x": 18,
      "y": 2,
      "s": 0
    }, {
      "x": 19,
      "y": 2,
      "s": 0
    }],
    "label": ",alpha=third-trace,beta=baz,gamma=qux,",
    "params": {
      "alpha": "second-trace",
      "beta": "baz",
      "gamma": "qux",
    }
  }],
  "digests": [{
    "digest": "ce0a9d2b546b25e00e39a33860cb72b6",
    "status": "untriaged"
  }, {
    "digest": "34e87ca0f753cf4c884fa01af6c08be9",
    "status": "positive"
  }, {
    "digest": "8ee9a2c61e9f12e6243f07423302f26a",
    "status": "untriaged"
  }, {
    "digest": "6174fc17b06e6ff9e383da3f6952ad68",
    "status": "positive"
  }, {
    "digest": "dcccd6998b47f60ab28dcff17ae57ed2",
    "status": "untriaged"
  }, {
    "digest": "92d9faf80a25750629118018716387df",
    "status": "untriaged"
  }, {
    "digest": "1bc4771dcee95d97b2758a1e1945cc40",
    "status": "untriaged"
  }, {
    "digest": "a9f4c341392618fad087060a0e69f170",
    "status": "untriaged"
  }, {
    "digest": "9522095d651fd5e6572904a1c13fb91c",
    "status": "untriaged"
  }, {
    "digest": "8ad66f50b755d82cd1c08b22e984bbef",
    "status": "untriaged"
  }]
};

export const commits = [{
  "commit_time": 1576186931,
  "hash": "46a331b93f54d8b3bce88792dd8679beef11a751",
  "author": "Alpha (alpha@example.com)"
}, {
  "commit_time": 1576186932,
  "hash": "1521e6b24c19f30eda383bb00b26862894ae9182",
  "author": "Beta (beta@example.com)"
}, {
  "commit_time": 1576189965,
  "hash": "f46d5ca49221113497d41e8f2a3c0c59151f4010",
  "author": "Gamma (gamma@example.com)"
}, {
  "commit_time": 1576190315,
  "hash": "dcd8e9389d8aa79a389aebad570d340ec012f367",
  "author": "Beta (beta@example.com)"
}, {
  "commit_time": 1576191335,
  "hash": "2fc9fa6d08df3b12c764d88f4458d28d4352de9b",
  "author": "Epsilon (epsilon@example.com)"
}, {
  "commit_time": 1576192005,
  "hash": "4d3b4a1bf31afb9d50ac84221a5852fea29a30df",
  "author": "Beta (beta@example.com)"
}, {
  "commit_time": 1576197935,
  "hash": "0678df30b5a56375ff6a4c21e6f0ecadb3493b7c",
  "author": "Delta (delta@example.com)"
}, {
  "commit_time": 1576200535,
  "hash": "39cdc37bdd0fd63556357a86b43f83fa4211ce0f",
  "author": "Beta (beta@example.com)"
}, {
  "commit_time": 1576211413,
  "hash": "252a03454d382b387c8f42aa75cfd63756816713",
  "author": "Beta (beta@example.com)"
}, {
  "commit_time": 1576213625,
  "hash": "415bce89a49abb6f53b2d3634159f6d304c8c8b5",
  "author": "Beta (beta@example.com)"
}, {
  "commit_time": 1576213773,
  "hash": "7fb7134e7d946d80741f779cfdb10cfd40a1f7a3",
  "author": "Beta (beta@example.com)"
}, {
  "commit_time": 1576246423,
  "hash": "d0840ecf583171e55025d2808dba017910b7a54f",
  "author": "Zeta (zeta@example.com)"
}, {
  "commit_time": 1576255153,
  "hash": "81b98978bced13406df91c2f5917cc2b82772f1e",
  "author": "Eta (Eta@example.com)"
}, {
  "commit_time": 1576258473,
  "hash": "a6069a154d66b2620bea1907b0eebf5d1afd02e7",
  "author": "Theta (Theta@example.com)"
}, {
  "commit_time": 1576260733,
  "hash": "1c5be7b19707c54ff859aa9f834a92e14d6ab5b9",
  "author": "Epsilon (epsilon@example.com)"
}, {
  "commit_time": 1576260973,
  "hash": "ab51c2ce0884a2bb1693d0f15d9eb674800e18ba",
  "author": "Iota (iota@example.com)"
}, {
  "commit_time": 1576262533,
  "hash": "c9b4d279d235c8db48875f0d0854bfe25c631ff6",
  "author": "Kappa (kappa@example.com)"
}, {
  "commit_time": 1576264923,
  "hash": "1cc767bd0d915bd0f3f5b40dcb282367c9fd9271",
  "author": "Gamma (gamma@example.com)"
}, {
  "commit_time": 1576265403,
  "hash": "a072b7b2758d644fbd5483a9716f581d270c7560",
  "author": "Lambda (lambda@example.com)"
}, {
  "commit_time": 1576265853,
  "hash": "17e7dfa37734347215f0b6bacb72c06ec85dbfdc",
  "author": "Epsilon (epsilon@example.com)"
}];
