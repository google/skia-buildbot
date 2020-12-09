import { PageSet } from '../json';

export const pageSets: PageSet[] = [
  { key: '100k', description: 'Top 100K (with desktop user-agent)' },
  { key: 'Mobile100k', description: 'Top 100K (with mobile user-agent)' },
  { key: '10k', description: 'Top 10K (with desktop user-agent)' },
  { key: 'Mobile10k', description: 'Top 10K (with mobile user-agent)' },
  { key: 'Dummy1k', description: 'Top 1K (with desktop user-agent, for testing, hidden from Runs History by default)' },
  { key: 'DummyMobile1k', description: 'Top 1K (with mobile user-agent, for testing, hidden from Runs History by default)' },
  { key: 'All', description: 'Top 1M (with desktop user-agent)' },
  { key: 'VoltMobile10k', description: 'Volt 10K (with mobile user-agent)' }];
