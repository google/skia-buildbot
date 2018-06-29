import '../styles/buttons'

import { $$ } from 'common-sk/modules/dom'
import { errorMessage } from './errorMessage'

import './index.js'

$$('#test_error_toast').addEventListener('click', e => errorMessage('Testing'));
