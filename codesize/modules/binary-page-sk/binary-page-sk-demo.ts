import './index';
import fetchMock from 'fetch-mock';
import { BinaryRPCRequest, BinaryRPCResponse } from '../rpc_types';
import { BinaryPageSk } from './binary-page-sk';
import { CodesizeScaffoldSk } from '../codesize-scaffold-sk/codesize-scaffold-sk';

const fakeRpcDelayMillis = 300;

const fakeRPCResponse: BinaryRPCResponse = {
  metadata: {
    version: 1,
    timestamp: '2022-02-15T23:57:57Z',
    swarming_task_id: '59189d06df0e9611',
    swarming_server: 'https://chromium-swarm.appspot.com',
    task_id: 'rG2vO5FkQOODEHgHf7W8',
    task_name: 'CodeSize-dm-Debian10-Clang-x86_64-Debug',
    compile_task_name: 'Build-Debian10-Clang-x86_64-Debug',
    binary_name: 'dm',
    bloaty_cipd_version: 'version:1',
    bloaty_args: ['build/dm', '-d', 'compileunits,symbols', '-n', '0', '--tsv'],
    patch_issue: '',
    patch_server: '',
    patch_set: '',
    repo: 'https://skia.googlesource.com/skia.git',
    revision: '34e3b35eb460e8668bb063adeefdc1fed857d075',
    commit_timestamp: '2022-02-15T23:42:43Z',
    author: 'Alice (alice@google.com)',
    subject: '[codesize] Define more CodeSize tasks for testing purposes (both for CQ and waterfall).',
  },
  rows: [ // This is based on the first few hundred rows of production data on Aug 17 2022
    { name: 'ROOT', parent: '', size: 0 },
    { name: 'third_party', parent: 'ROOT', size: 0 },
    { name: 'third_party/externals', parent: 'third_party', size: 0 },
    { name: 'third_party/externals/harfbuzz', parent: 'third_party/externals', size: 0 },
    { name: 'third_party/externals/harfbuzz/src', parent: 'third_party/externals/harfbuzz', size: 0 },
    { name: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', parent: 'third_party/externals/harfbuzz/src', size: 0 },
    { name: 'OT::OffsetTo<>::sanitize<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 11221 },
    { name: 'OT::ArrayOf<>::sanitize<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 6835 },
    { name: 'OT::ArrayOf<>::sanitize_shallow()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 6421 },
    { name: 'hb_sanitize_context_t::try_set<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 4563 },
    { name: 'hb_sanitize_context_t::check_struct<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 4440 },
    { name: 'hb_sanitize_context_t::check_range<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 3649 },
    { name: 'OT::OffsetTo<>::neuter()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 3306 },
    { name: 'OT::hb_kern_machine_t<>::kern()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 3290 },
    { name: 'OT::hb_accelerate_subtables_context_t::dispatch<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 3264 },
    { name: 'OT::hb_accelerate_subtables_context_t::hb_applicable_t::init<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 3027 },
    { name: 'OT::Layout::GSUB_impl::Ligature<>::apply()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 2283 },
    { name: 'hb_sanitize_context_t::check_array<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 2001 },
    { name: 'OT::OffsetTo<>::operator()()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1845 },
    { name: 'OT::Layout::GPOS_impl::PairPosFormat2_4<>::apply()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1611 },
    { name: 'OT::Layout::GSUB_impl::Sequence<>::apply()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1461 },
    { name: 'hb_ot_map_t::apply<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1448 },
    { name: 'OT::OffsetTo<>::sanitize_shallow()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1350 },
    { name: 'OT::Layout::GPOS_impl::PairSet<>::apply()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1328 },
    { name: 'OT::Layout::GPOS_impl::CursivePosFormat1::apply()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1277 },
    { name: 'OT::hb_accelerate_subtables_context_t::apply_to<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1266 },
    { name: 'OT::Layout::GPOS_impl::MarkMarkPosFormat1_2<>::apply()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1084 },
    { name: 'AAT::KerxTable<>::apply()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1071 },
    { name: 'OT::match_input<>()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1048 },
    { name: 'OT::Layout::GPOS_impl::MarkBasePosFormat1_2<>::apply()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1033 },
    { name: 'OT::Layout::GPOS_impl::MarkLigPosFormat1_2<>::apply()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 1003 },
    { name: 'OT::Layout::GPOS_impl::PairPosFormat2_4<>::sanitize()', parent: 'third_party/externals/harfbuzz/src/hb-ot-layout.cc', size: 980 },
    { name: 'skia', parent: 'ROOT', size: 0 },
    { name: 'skia/src', parent: 'skia', size: 0 },
    { name: 'skia/src/core', parent: 'skia/src', size: 0 },
    { name: 'skia/src/core/SkOpts.cpp', parent: 'skia/src/core', size: 0 },
    { name: '(anonymous namespace)::xfer_aa<>()', parent: 'skia/src/core/SkOpts.cpp', size: 7137 },
    { name: '(anonymous namespace)::Sk4pxXfermode<>::xfer32()', parent: 'skia/src/core/SkOpts.cpp', size: 5972 },
    { name: 'sse2::interpret_skvm()', parent: 'skia/src/core/SkOpts.cpp', size: 5793 },
    { name: '_ZN4sse24lowpL17bilerp_clamp_8888EmPPvmmDv8_tS3_S3_S3_S3_S3_S3_S3_', parent: 'skia/src/core/SkOpts.cpp', size: 2518 },
    { name: 'sse2::blit_mask_d32_a8()', parent: 'skia/src/core/SkOpts.cpp', size: 1804 },
    { name: 'skia/src/core/SkPictureData.cpp', parent: 'skia/src/core', size: 0 },
    { name: 'SkPictureData::parseBuffer()', parent: 'skia/src/core/SkPictureData.cpp', size: 1981 },
    { name: 'SkPictureData::readBuffer()', parent: 'skia/src/core/SkPictureData.cpp', size: 156 },
  ],
};

fetchMock.post(
  (url, opts) => {
    const request = JSON.parse(opts.body?.toString() || '') as BinaryRPCRequest;
    return url === '/rpc/binary/v1'
      && request.commit === fakeRPCResponse.metadata.revision
      && request.patch_issue === fakeRPCResponse.metadata.patch_issue
      && request.patch_set === fakeRPCResponse.metadata.patch_set
      && request.binary_name === fakeRPCResponse.metadata.binary_name
      && request.compile_task_name === fakeRPCResponse.metadata.compile_task_name;
  },
  () => new Promise(
    (resolve) => setTimeout(
      () => resolve(JSON.stringify(fakeRPCResponse)),
      fakeRpcDelayMillis,
    ),
  ),
);

const queryString = `?commit=${fakeRPCResponse.metadata.revision}&`
  + `patch_issue=${fakeRPCResponse.metadata.patch_issue}&`
  + `patch_set=${fakeRPCResponse.metadata.patch_set}&`
  + `binary_name=${fakeRPCResponse.metadata.binary_name}&`
  + `compile_task_name=${fakeRPCResponse.metadata.compile_task_name}`;
window.history.pushState(null, '', queryString);

// Add the page under test only after all RPCs are mocked out.
const scaffold = new CodesizeScaffoldSk();
document.body.appendChild(scaffold);
scaffold.appendChild(new BinaryPageSk());
