import './index';
import { assert } from 'chai';
import { SheriffConfigsDryRunSk } from './sheriff-configs-dry-run-sk';
import { setUpElementUnderTest } from '../../../infra-sk/modules/test_util';

describe('sheriff-configs-dry-run-sk', () => {
  const newInstance = setUpElementUnderTest<SheriffConfigsDryRunSk>('sheriff-configs-dry-run-sk');

  let element: SheriffConfigsDryRunSk;

  beforeEach(async () => {
    element = newInstance();
    await element.updateComplete;
  });

  it('renders initial state', () => {
    assert.isNotNull(element);
    assert.include(element.textContent, 'Sheriff Config Dry Run');
    assert.include(element.textContent, 'Builder');
    assert.include(element.textContent, 'Protobuf Text');
    assert.include(element.textContent, 'Anomaly Configs');
    assert.include(element.textContent, 'Rules');
  });

  it('toggles view mode to proto and back to builder', async () => {
    // Switch to proto view
    const protoRadio = element.querySelector<HTMLInputElement>('md-radio[value="proto"]')!;
    protoRadio.click();
    await element.updateComplete;
    // Second await for toggleViewMode update cycle.
    await element.updateComplete;

    assert.equal((element as any).viewMode, 'proto');
    const protoTextField = element.querySelector<HTMLInputElement>('md-outlined-text-field')!;
    assert.include(protoTextField.value, 'subscriptions {'); // Check for proto text content

    // Switch back to builder view
    const builderRadio = element.querySelector<HTMLInputElement>('md-radio[value="builder"]')!;
    builderRadio.click();
    await element.updateComplete;
    // Second await for toggleViewMode update cycle.
    await element.updateComplete;

    assert.equal((element as any).viewMode, 'builder');
    assert.include(element.textContent, 'Anomaly Configs'); // Check for builder content
  });

  it('generates proto text correctly', async () => {
    (element as any).configName = 'TestConfig';
    (element as any).contactEmail = 'test@example.com';
    (element as any).bugComponent = '12345';
    (element as any).threshold = 3.0;
    (element as any).step = 'PERCENT_STEP';
    (element as any).radius = 5;
    (element as any).sparse = false;
    (element as any).rulesMatch = 'key=value\nkey2=value2';
    (element as any).rulesExclude = 'exclude=this';
    (element as any).action = 'BISECT';

    await element.updateComplete;

    const expectedProto = `subscriptions {
  name: "TestConfig"
  contact_email: "test@example.com"
  bug_component: "12345"
  anomaly_configs {
    threshold: 3
    step: PERCENT_STEP
    radius: 5
    sparse: False
    rules: {
      match: [
        "key=value",
        "key2=value2"
      ]
      exclude: [
        "exclude=this"
      ]
    }
    action: BISECT
  }
}`;
    assert.equal(element.getProto(), expectedProto);
  });

  it('imports proto text correctly', async () => {
    const protoText = `subscriptions {
  name: "ImportedConfig"
  contact_email: "import@example.com"
  bug_component: "67890"
  anomaly_configs {
    threshold: 4.5
    step: ABSOLUTE_STEP
    radius: 12
    sparse: True
    rules: {
      match: [
        "imported_key=value"
      ]
      exclude: [
        "imported_exclude=this"
      ]
    }
    action: REPORT
  }
}`;
    element.importProto(protoText);
    await element.updateComplete;

    assert.equal((element as any).configName, 'ImportedConfig');
    assert.equal((element as any).contactEmail, 'import@example.com');
    assert.equal((element as any).bugComponent, '67890');
    assert.equal((element as any).threshold, 4.5);
    assert.equal((element as any).step, 'ABSOLUTE_STEP');
    assert.equal((element as any).radius, 12);
    assert.isTrue((element as any).sparse);
    assert.equal((element as any).rulesMatch, 'imported_key=value');
    assert.equal((element as any).rulesExclude, 'imported_exclude=this');
    assert.equal((element as any).action, 'REPORT');
  });

  it('generates compound proto text correctly', async () => {
    (element as any).configName = 'CompoundConfig';
    (element as any).contactEmail = 'compound@example.com';
    (element as any).bugComponent = '54321';
    (element as any).ruleType = 'compound';
    (element as any).compoundOp = 'OR';
    (element as any).compoundRules = [
      { step: 'COHEN_STEP', threshold: 2.5 },
      { step: 'ABSOLUTE_STEP', threshold: 5.0 },
    ];
    (element as any).radius = 10;
    (element as any).sparse = true;
    (element as any).rulesMatch = 'compound_key=value';
    (element as any).rulesExclude = '';
    (element as any).action = 'TRIAGE';

    await element.updateComplete;

    const expectedProto = `subscriptions {
  name: "CompoundConfig"
  contact_email: "compound@example.com"
  bug_component: "54321"
  anomaly_configs {
    radius: 10
    sparse: True
    detection_rule {
      complex_rule {
        op: OR
        rules {
          simple_rule {
            step: COHEN_STEP
            threshold: 2.5
          }
        }
        rules {
          simple_rule {
            step: ABSOLUTE_STEP
            threshold: 5
          }
        }
      }
    }
    rules: {
      match: [
        "compound_key=value"
      ]
    }
    action: TRIAGE
  }
}`;
    assert.equal(element.getProto(), expectedProto);
  });

  it('imports compound proto text correctly', async () => {
    const protoText = `subscriptions {
  name: "ImportedCompound"
  contact_email: "import_comp@example.com"
  bug_component: "99999"
  anomaly_configs {
    radius: 15
    sparse: False
    detection_rule {
      complex_rule {
        op: AND
        rules {
          simple_rule {
            step: PERCENT_STEP
            threshold: 1.5
          }
        }
        rules {
          simple_rule {
            step: CONST_STEP
            threshold: 100
          }
        }
      }
    }
    rules: {
      match: [
        "comp_match=val"
      ]
    }
    action: NOACTION
  }
}`;
    element.importProto(protoText);
    await element.updateComplete;

    assert.equal((element as any).configName, 'ImportedCompound');
    assert.equal((element as any).contactEmail, 'import_comp@example.com');
    assert.equal((element as any).bugComponent, '99999');
    assert.equal((element as any).ruleType, 'compound');
    assert.equal((element as any).compoundOp, 'AND');
    assert.deepEqual((element as any).compoundRules, [
      { step: 'PERCENT_STEP', threshold: 1.5 },
      { step: 'CONST_STEP', threshold: 100 },
    ]);
    assert.equal((element as any).radius, 15);
    assert.isFalse((element as any).sparse);
    assert.equal((element as any).rulesMatch, 'comp_match=val');
    assert.equal((element as any).rulesExclude, '');
    assert.equal((element as any).action, 'NOACTION');
  });

  it('rejects malformed proto text with typos', async () => {
    const protoText = `subscriptions {
  name: "InvalidConfig"
  anomaly_configs {
    detection_rule {
      complex_rule {
        op: AND
        rules {
          simple_rulue {
            step: PERCENT_STEP
            threshold: 1.5
          }
        }
      }
    }
  }
}`;
    const success = element.importProto(protoText);
    assert.isFalse(success);
  });

  it('rejects proto text with invalid step enum', async () => {
    const protoText = `subscriptions {
  name: "InvalidStepConfig"
  anomaly_configs {
    threshold: 2.5
    step: COHEN_STEPP
    radius: 8
    sparse: True
    rules: {
      match: [
        "key=value"
      ]
    }
  }
}`;
    const success = element.importProto(protoText);
    assert.isFalse(success);
  });
});
