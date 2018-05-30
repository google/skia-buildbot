import 'elements-sk/buttons'

import { $$ } from '../dom'
import { errorMessage } from '../errorMessage'

import './index.js'

$$('#test_error_toast').addEventListener('click', e => errorMessage('Testing'));
