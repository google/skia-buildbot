import './index';
import '../../../infra-sk/modules/theme-chooser-sk';
import { $$ } from 'common-sk/modules/dom';
import { fetchMock } from 'fetch-mock';
import { buildsJson } from './test_data';

fetchMock.post('begin:/_/get_chromium_build_tasks', buildsJson);
const selector = document.createElement('chromium-build-selector-sk');
$$('#container').appendChild(selector);
