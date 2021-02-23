/* eslint-disable dot-notation */
import './index';
import { assert } from 'chai';
import { numPredefinedUniforms, ShaderNode } from './index';
import { CanvasKit } from '../../build/canvaskit/canvaskit';

// eslint-disable-next-line @typescript-eslint/no-var-requires
const CanvasKitInit = require('../../build/canvaskit/canvaskit.js');

let canvasKit: CanvasKit | null = null;

const getCanvasKit = async (): Promise<CanvasKit> => {
  if (canvasKit) {
    return canvasKit;
  }
  canvasKit = await CanvasKitInit({ locateFile: (file: string) => `https://particles.skia.org/dist/${file}` });
  return canvasKit!;
};

const createShaderNode = async (): Promise<ShaderNode> => {
  const ck = await getCanvasKit();
  return new ShaderNode(ck);
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
    node.setScrap({
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
    node.setScrap({
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

    // Changing the code means we need to save.
    const originalCode = node.shaderCode;
    assert.isFalse(node.needsSave());
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
    node.setScrap({
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

    assert.deepEqual(node.compileErrorLineNumbers, [4]);
    node.compileErrorMessage.startsWith('error: 4:');
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
});
