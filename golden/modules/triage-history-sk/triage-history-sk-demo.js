import './index';

Date.now = () => new Date('2020-03-10T01:01:01-04:00');

const ele = document.querySelector('#with_data');
ele.history = [
  {
    user: 'test@example.com',
    ts: '2020-03-09T12:56:08.000000000-04:00',
  },
];
