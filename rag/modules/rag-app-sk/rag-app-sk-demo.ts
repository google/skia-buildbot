import fetchMock from 'fetch-mock';

fetchMock.get('glob:/historyrag/v1/topics?*', {
  topics: [
    {
      topicId: 1,
      topicName: 'Database Migration',
      summary: 'Discussion about migrating from MySQL to Spanner.',
    },
    {
      topicId: 2,
      topicName: 'Authentication Bug',
      summary: 'Fixing the token expiration issue in auth service.',
    },
    {
      topicId: 3,
      topicName: 'UI Refactoring',
      summary: 'Updating the dashboard to use Material 3.',
    },
  ],
});

fetchMock.get('glob:/historyrag/v1/topic_details?*', {
  topics: [
    {
      topicId: 1,
      topicName: 'Database Migration',
      summary:
        'Discussion about migrating from MySQL to Spanner. The migration involves schema changes and data backfill.',
      codeChunks: ['func Migrate() {\n  // Connect to Spanner\n}', 'CREATE TABLE users (...)'],
    },
  ],
});
