/* eslint-disable dot-notation */
import './index';
import fetchMock, { MockRequest, MockResponse } from 'fetch-mock';
import { assert } from 'chai';
import { CanvasKit, CanvasKitInit as CKInit } from 'canvaskit-wasm';
import {
  childShaderArraysDiffer,
  childShadersAreDifferent,
  defaultChildShaderScrapHashOrName, defaultImageURL, defaultScrapBody, numPredefinedUniforms, ShaderNode,
} from './index';
import { ChildShader, ScrapBody, ScrapID } from '../json';

// It is assumed that canvaskit.js has been loaded and this symbol is available globally.
declare const CanvasKitInit: typeof CKInit;

let canvasKit: CanvasKit | null = null;

const getCanvasKit = async (): Promise<CanvasKit> => {
  if (canvasKit) {
    return canvasKit;
  }
  canvasKit = await CanvasKitInit({ locateFile: (file: string) => `/canvaskit_assets/${file}` });
  if (!canvasKit) {
    throw new Error('Could not load CanvasKit');
  }
  return canvasKit;
};

const createShaderNode = async (): Promise<ShaderNode> => {
  const ck = await getCanvasKit();
  const node = new ShaderNode(ck);
  await node.setScrap(defaultScrapBody);
  return node;
};

const createShaderNodeWithChildShader = async (): Promise<ShaderNode> => {
  const ck = await getCanvasKit();
  const node = new ShaderNode(ck);

  const childScrapBody: ScrapBody = {
    Body: `half4 main(vec2 fragcoord) {
      return half4(0, 1, 0, 1);
    }`,
    Type: 'sksl',
    SKSLMetaData: {
      Children: [],
      ImageURL: '',
      Uniforms: [],
    },
  };
  fetchMock.get(`/_/load/${defaultChildShaderScrapHashOrName}`, childScrapBody);

  const scrapBodyWithChild: ScrapBody = {
    Body: `half4 main(vec2 fragcoord) {
      return half4(0, 1, 0, 1);
    }`,
    Type: 'sksl',
    SKSLMetaData: {
      Children: [{
        UniformName: 'childShader',
        ScrapHashOrName: defaultChildShaderScrapHashOrName,
      },
      ],
      ImageURL: '',
      Uniforms: [],
    },
  };
  await node.setScrap(scrapBodyWithChild);
  await fetchMock.flush();
  fetchMock.restore();

  return node;
};

describe('ShaderNode', async () => {
  it('constructor builds with a default shader', async () => {
    const node = await createShaderNode();
    node.compile();

    // Confirm that all the post-compile pre-calculations are done correctly.
    assert.equal(node.getUniformCount(), numPredefinedUniforms, 'The default shader doesn\'t have any user uniforms.');
    assert.equal(node.getUniform(0).name, 'iResolution', 'Confirm the predefined shaders show up in the uniforms.');
    assert.equal(node.getUniformFloatCount(), node.numPredefinedUniformValues, 'These are equal because the default shader has 0 user uniforms.');
    assert.isNotNull(node['uniformsMallocObj'], "We Malloc'd");
    assert.equal(node.numPredefinedUniformValues, 11, 'The number of predefined uniform values is calculated after compile() is called. This value will change if predefinedUniforms is changed.');
    assert.deepEqual(node.compileErrorLineNumbers, []);
    assert.equal(node.compileErrorMessage, '');
  });

  it('updates all values when a new shader is compiled.', async () => {
    const node = await createShaderNode();
    node.compile();
    assert.isNotNull(node.getShader([]));

    // Check our starting values.
    assert.equal(node.getUniformCount(), numPredefinedUniforms, 'The default shader doesn\'t have any user uniforms.');
    assert.equal(node.getUniform(0).name, 'iResolution', 'Confirm the predefined shaders show up in the uniforms.');
    assert.equal(node.getUniformFloatCount(), node.numPredefinedUniformValues, 'These are equal because the default shader has 0 user uniforms.');

    // Set code that has a user uniform, in this case with 4 floats.
    await node.setScrap({
      Type: 'sksl',
      Body: `uniform float4 iColorWithAlpha;

      half4 main(float2 fragCoord) {
        return half4(iColorWithAlpha);
      }
      `,
      SKSLMetaData: {
        Children: [],
        ImageURL: '',
        Uniforms: [1, 0, 1, 0],
      },
    });
    node.compile();
    assert.isNotNull(node.getShader([0, 0, 0, 0]));

    // Confirm that all the post-compile pre-calculations are done correctly for the new shader.
    assert.equal(node.getUniformCount(), numPredefinedUniforms + 1, 'The new shader has 1 user uniform.');
    assert.equal(node.getUniform(0).name, 'iResolution', 'Confirm the predefined shaders show up in the uniforms.');
    assert.equal(node.getUniformFloatCount(), node.numPredefinedUniformValues + 4, 'The user uniform contributes 4 floats to the total.');
  });

  it('correctly indicates when run() needs to be called.', async () => {
    const node = await createShaderNode();
    node.compile();

    assert.isFalse(node.needsCompile(), 'Should not need a run immediately after a call to compile().');

    const originalCode = node.shaderCode;
    node.shaderCode += '\n';
    assert.isTrue(node.needsCompile(), 'Needs compile when code has changed.');
    node.shaderCode = originalCode;
    assert.isFalse(node.needsCompile(), 'No longer needs a compile when change is undone.');
  });

  it('correctly indicates when save() needs to be called.', async () => {
    const node = await createShaderNode();
    node.compile();

    const startingUniformValues = [1, 0, 1, 0];
    const modifiedUniformValues = [1, 1, 1, 1];

    // Set code that has a user uniform, in this case with 4 floats, because
    // saving is not only indicated when the code changes, but when the user
    // uniforms change.
    await node.setScrap({
      Type: 'sksl',
      Body: `uniform float4 iColorWithAlpha;

      half4 main(float2 fragCoord) {
        return half4(iColorWithAlpha);
      }
      `,
      SKSLMetaData: {
        Children: [],
        ImageURL: '',
        Uniforms: startingUniformValues,
      },
    });
    node.compile();

    const originalCode = node.shaderCode;
    assert.isFalse(node.needsSave(), 'No need to save at the start.');
    // Changing the code means we need to save.

    node.shaderCode += '\n';
    assert.isTrue(node.needsSave(), 'Needs save if code changed.');
    node.shaderCode = originalCode;
    assert.isFalse(node.needsSave(), "Doesn't need save when code restored.");

    // Also changing the user uniform values means we need to save.
    node.currentUserUniformValues = modifiedUniformValues;
    assert.isTrue(node.needsSave(), 'Needs save if uniform values changed.');
    node.currentUserUniformValues = startingUniformValues;
    assert.isFalse(node.needsSave(), "Doesn't need save if uniform values restored.");
  });

  it('reports compiler errors', async () => {
    const node = await createShaderNode();
    node.compile();
    await node.setScrap({
      Type: 'sksl',
      Body: `uniform float4 iColorWithAlpha;

      half4 main(float2 fragCoord) {
        return half4(iColorWithAlpha) // Missing trailing semicolon.
      }
      `,
      SKSLMetaData: {
        Children: [],
        ImageURL: '',
        Uniforms: [1, 0, 1, 0],
      },
    });
    node.compile();

    assert.deepEqual(node.compileErrorLineNumbers, [5]);
    node.compileErrorMessage.startsWith("error: 5: expected ';', but found '}'");
  });

  it('makes a copy of the ScrapBody', async () => {
    const node = await createShaderNode();
    const startScrap = node.getScrap();
    assert.isNotEmpty(startScrap.Body);
    startScrap.Body = '';
    // Confirm we haven't changed the original scrap.
    assert.isNotEmpty(node['body']!.Body);
  });

  it('always starts with non-null input image', async () => {
    const node = await createShaderNode();
    assert.isNotNull(node.inputImageElement);
  });

  it('protects against unsafe URLs', async () => {
    const node = await createShaderNode();
    node['currentImageURL'] = 'data:foo';
    assert.equal(node.getCurrentImageURL(), 'data:foo');
    assert.equal(node.getSafeImageURL(), defaultImageURL);
  });

  it('reverts to empty image URL if image fails to load.', async () => {
    const node = await createShaderNode();
    await node.setCurrentImageURL('/dist/some-unknown-image.png');
    assert.equal(node.getCurrentImageURL(), '');
  });

  describe('child shader', () => {
    it('is created on loadScrap', async () => {
      const node = await createShaderNodeWithChildShader();
      assert.equal(1, node.children.length);
      assert.equal(defaultChildShaderScrapHashOrName, node.children[0]['scrapID']);
      assert.equal(node.getChildShader(0).UniformName, 'childShader');
    });

    it('can be removed', async () => {
      const node = await createShaderNodeWithChildShader();
      node.removeChildShader(0);
      assert.equal(0, node.children.length);
    });

    it('throws on out of bounds when removing shader', async () => {
      const node = await createShaderNodeWithChildShader();
      assert.throws(() => {
        node.removeChildShader(2);
      });
    });

    it('throws on out of bounds when accessing child shader', async () => {
      const node = await createShaderNodeWithChildShader();
      assert.throws(() => {
        node.getChildShader(2);
      });
    });

    it('can be appended', async () => {
      const node = await createShaderNode();

      const childScrapBody: ScrapBody = {
        Body: `half4 main(vec2 fragcoord) {
          return half4(0, 1, 0, 1);
        }`,
        Type: 'sksl',
        SKSLMetaData: {
          Children: [],
          ImageURL: '',
          Uniforms: [],
        },
      };
      fetchMock.get(`/_/load/${defaultChildShaderScrapHashOrName}`, childScrapBody);

      await node.appendNewChildShader();
      assert.equal(1, node.children.length);
      assert.equal(defaultChildShaderScrapHashOrName, node.children[0]['scrapID']);
      await fetchMock.flush();
      assert.isTrue(fetchMock.done());
      fetchMock.restore();
    });

    it('has a name', async () => {
      const node = await createShaderNodeWithChildShader();
      assert.equal(node.getChildShaderUniformName(0), 'childShader');
    });

    it('has uniform declarations', async () => {
      const node = await createShaderNodeWithChildShader();
      assert.equal(node.getChildShaderUniforms(), 'uniform shader childShader;');
    });

    it('has a name that can be changed', async () => {
      const node = await createShaderNodeWithChildShader();
      const newUniformName = 'someNewName';

      const childScrapBody: ScrapBody = {
        Body: `half4 main(vec2 fragcoord) {
          return half4(0, 1, 0, 1);
        }`,
        Type: 'sksl',
        SKSLMetaData: {
          Children: [],
          ImageURL: '',
          Uniforms: [],
        },
      };
      fetchMock.get(`/_/load/${defaultChildShaderScrapHashOrName}`, childScrapBody);

      await node.setChildShaderUniformName(0, newUniformName);
      assert.equal(node.getChildShaderUniformName(0), newUniformName);

      await fetchMock.flush();
      assert.isTrue(fetchMock.done());
      fetchMock.restore();
    });

    it('raises on invalid child shader names', async () => {
      const node = await createShaderNodeWithChildShader();
      await node.setChildShaderUniformName(0, 'this is an invalid uniform name because it contains spaces')
        .then(() => assert.fail())
        .catch((err: Error) => assert.match(err.message, /Invalid uniform name/));
    });

    it('raises on out of bounds', async () => {
      const node = await createShaderNodeWithChildShader();
      await node.setChildShaderUniformName(1, 'aNewName')
        .then(() => assert.fail())
        .catch((err: Error) => assert.match(err.message, /does not exist/));
    });

    it('saves depth first', async () => {
      const parentNodeSavedID = 'parentNodeSavedID';
      const childNodeSavedID = 'childNodeSavedID';
      const node = await createShaderNodeWithChildShader();

      // The save endpoint should be called twice, the first time from the child
      // node, and the second time from the parent node.
      const callOrder = [childNodeSavedID, parentNodeSavedID];
      const bodiesSent = [
        '{"Body":"half4 main(vec2 fragcoord) {\\n      return half4(0, 1, 0, 1);\\n    }","Type":"sksl","SKSLMetaData":{"Uniforms":[],"ImageURL":"","Children":[]}}',
        '{"Body":"half4 main(vec2 fragcoord) {\\n      return half4(0, 1, 0, 1);\\n    }","Type":"sksl","SKSLMetaData":{"Uniforms":[],"ImageURL":"","Children":[{"UniformName":"childShader","ScrapHashOrName":"childNodeSavedID"}]}}',
      ];
      let call = 0;
      fetchMock.post('/_/save/', (url: string, opts: MockRequest): MockResponse => {
        const { body } = opts;
        assert.equal(body, bodiesSent[call]);
        const resp: ScrapID = {
          Hash: callOrder[call],
        };
        call++;
        return resp;
      }, {
        sendAsJson: true,
      });

      const newID = await node.saveScrap();

      await fetchMock.flush();
      assert.isTrue(fetchMock.done());
      fetchMock.restore();

      assert.equal(newID, parentNodeSavedID);
    });
  });

  describe('childShadersAreDifferent', () => {
    it('detects differences', () => {
      const a: ChildShader = {
        UniformName: 'foo',
        ScrapHashOrName: '@someName',
      };
      const b: ChildShader = {
        UniformName: 'bar',
        ScrapHashOrName: '@someName',
      };
      const c: ChildShader = {
        UniformName: 'foo',
        ScrapHashOrName: '@someDifferentName',
      };

      assert.isTrue(childShadersAreDifferent(a, b));
      assert.isTrue(childShadersAreDifferent(b, c));
      assert.isTrue(childShadersAreDifferent(a, c));
      assert.isFalse(childShadersAreDifferent(a, a));
    });
  });

  describe('childShaderArraysDiffer', () => {
    it('handles empty arrays', () => {
      assert.isFalse(childShaderArraysDiffer([], []));
    });

    it('handles different sized arrays', () => {
      const a: ChildShader = {
        UniformName: 'foo',
        ScrapHashOrName: '@someName',
      };

      assert.isTrue(childShaderArraysDiffer([], [a]));
      assert.isTrue(childShaderArraysDiffer([a], []));
      assert.isTrue(childShaderArraysDiffer([a], [a, a]));
      assert.isFalse(childShaderArraysDiffer([a], [a]));
    });
  });
});
