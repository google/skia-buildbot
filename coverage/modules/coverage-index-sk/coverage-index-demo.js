import './index.js'

// Can't use import fetch-mock because the library isn't quite set up
// correctly for it, and we get strange errors about "this" not being defined.
const fetchMock = require('fetch-mock')

var data = {
  list: [
    {
      info: {
        hash: "abcdef0123",
        author: "nobody whozit (nobody@example.com)",
        subject: "This is a really obnixiously long commit subject.  The author didn't read the guidlines on keeping it short."
      },
      combined: {
        name: "",
        total_lines: 20000,
        missed_lines: 6000,
      },
      jobs: [
        {
          name: "Test-Some-Config-Release",
          total_lines: 10000,
          missed_lines: 4500,
        },
        {
          name: "Test-Some-Config-Debug",
          total_lines: 11000,
          missed_lines: 4500,
        }
      ],
    },
    {
      info: {
        hash: "feedbar",
        author: "Nobody Whozit Junior (nobodyjr@example.com)",
        subject: "Terse commit",
      },
      combined: {
        name: "",
        total_lines: 20000,
        missed_lines: 6000,
      },
      jobs: [
        {
          name: "Test-Other-Config-Release",
          total_lines: 13000,
          missed_lines: 4500,
        },
        {
          name: "Test-Other-Config-Debug",
          total_lines: 8000,
          missed_lines: 4500,
        },
        {
          name: "Test-Some-Config-Release",
          total_lines: 9000,
          missed_lines: 4500,
        },
        {
          name: "Test-Some-Config-Debug-ASAN",
          total_lines: 7000,
          missed_lines: 4500,
        }
      ],
    },
    {
      info: {
        hash: "feedbar",
        author: "Nobody Whozit Junior (nobodyjr@example.com)",
        subject: "No combined",
      },
      jobs: [
        {
          name: "Test-Other-Config-Release",
          total_lines: 13000,
          missed_lines: 4500,
        },
        {
          name: "Test-Other-Config-Debug",
          total_lines: 8000,
          missed_lines: 4500,
        },
        {
          name: "Test-Some-Config-Release",
          total_lines: 9000,
          missed_lines: 4500,
        },
      ],
    },
  ],
};

fetchMock.get('/ingested', JSON.stringify(data));
