import './index';
import '../../../infra-sk/modules/app-sk';
import { $ } from 'common-sk/modules/dom';
import { testData } from './test-data';

import {
  CommandsSk,

} from './commands-sk';

$<CommandsSk>('commands-sk').forEach((command) => command.processCommands(testData));
