import './index.js'

    var items = [
      {
        name: "config",
        values: [
          {value:"565", weight: 20},
          {value:"8888", weight: 11},
        ],
      },
      {
        name: "cpu_or_gpu",
        values: [
          {value:"CPU", weight: 24},
          {value:"GPU", weight: 8},
        ],
      },
      {
        name: "arch",
        values: [
          {value:"x86", weight: 24},
          {value:"arm", weight: 8},
        ],
      },
    ];
    document.body.querySelector('word-cloud3-sk').items = items;
