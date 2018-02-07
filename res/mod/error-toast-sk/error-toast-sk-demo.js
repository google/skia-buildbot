import './index.js'
import 'skia-elements/buttons'
import { $ } from 'skia-elements/core'
import { errorMessage } from 'common/errorMessage'

$('test_error_toast').addEventListener('click', e => errorMessage('Testing'));
