import { assert } from 'chai';
import {
  Params,
  ParamSet,
  ReadOnlyParamSet,
  CommitNumber,
  TimestampSeconds,
  SerializesToString,
  ingest,
  Trace,
  TraceSet,
} from './index';

describe('json/nominal_types', () => {
  it('Params constructor returns the object', () => {
    const v = { a: 'b' };
    assert.strictEqual(Params(v), v as any);
  });

  it('ParamSet constructor returns the object', () => {
    const v = { a: ['b'] };
    assert.strictEqual(ParamSet(v), v as any);
  });

  it('ReadOnlyParamSet constructor returns the object', () => {
    const v = { a: ['b'] };
    assert.strictEqual(ReadOnlyParamSet(v), v as any);
  });

  it('CommitNumber constructor returns the number', () => {
    const v = 123;
    assert.strictEqual(CommitNumber(v), v as any);
  });

  it('TimestampSeconds constructor returns the number', () => {
    const v = 123456789;
    assert.strictEqual(TimestampSeconds(v), v as any);
  });

  it('SerializesToString constructor returns the string', () => {
    const v = '123';
    assert.strictEqual(SerializesToString(v), v as any);
  });

  it('CL constructor returns the string', () => {
    const v = 'abc';
    assert.strictEqual(ingest.CL(v), v as any);
  });

  it('Trace constructor returns the array', () => {
    const v = [1, 2, 3];
    assert.strictEqual(Trace(v), v as any);
  });

  it('TraceSet constructor returns the object', () => {
    const v = { a: Trace([1, 2, 3]) };
    assert.strictEqual(TraceSet(v), v as any);
  });
});
