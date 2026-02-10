// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import './index';
import { assert } from 'chai';
import { ErrorToastSk } from './error-toast-sk';
import { errorMessage } from '../errorMessage';
import { ToastSk } from '../toast-sk/toast-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('error-toast-sk', () => {
  const newInstance = setUpElementUnderTest<ErrorToastSk>('error-toast-sk');

  let errorToastSk: ErrorToastSk;

  beforeEach(() => {
    errorToastSk = newInstance();
  });

  it('can be cancelled by clicking on the close button', async () => {
    const childToast = errorToastSk.querySelector('toast-sk') as ToastSk;

    // Display the error toast and confirm it is displayed.
    errorMessage('This is an error message which should display forever', 0);
    await childToast.updateComplete;
    assert.isTrue(childToast.hasAttribute('shown'));

    // Click the close button.
    errorToastSk.querySelector('button')!.click();
    await childToast.updateComplete;
    assert.isFalse(childToast.hasAttribute('shown'));
  });
});
