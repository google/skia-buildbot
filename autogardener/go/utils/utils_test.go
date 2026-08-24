package utils

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeErrorText(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty",
			input:    "",
			expected: "",
		},
		{
			name:     "hex addresses",
			input:    "crash at 0x7fa2b8c9d0e1 with pc 0x88f2b4",
			expected: "crash at 0x... with pc 0x88f2b4",
		},
		{
			name:     "process and thread IDs",
			input:    "failed with pid 12345 thread 8427 LWP 456",
			expected: "failed with pid <ID> thread <ID> LWP <ID>",
		},
		{
			name:     "goroutines",
			input:    "goroutine 12 [running] or goroutine [42]",
			expected: "goroutine <ID> [running] or goroutine <ID>",
		},
		{
			name:     "swarming workdir prefix",
			input:    "error in /b/s/w/ir/cache/work/skia/src/core/SkCanvas.cpp:123",
			expected: "error in /<workdir>/skia/src/core/SkCanvas.cpp:123",
		},
		{
			name:     "kitchen workdir prefix",
			input:    "error in /b/s/w/ir/kitchen-workdir/recipe_bootstrap/infra/main.go",
			expected: "error in /<workdir>/infra/main.go",
		},
		{
			name:     "port numbers",
			input:    "cannot connect to 127.0.0.1:8080 or localhost:12345",
			expected: "cannot connect to <IP>:<port> or localhost:<port>",
		},
		{
			name:     "timestamps and dates",
			input:    "2026-08-19 19:20:13.407246 panic occurred",
			expected: "<DATE> <TIME> panic occurred",
		},
		{
			name:     "IP addresses",
			input:    "connection from 192.168.1.50 failed",
			expected: "connection from <IP> failed",
		},
		{
			name:     "log timestamps",
			input:    "E0819 11:48:24.557086    2338 step.go:163] exit status 2",
			expected: "E<DATE> <TIME> <TID> step.go:163] exit status 2",
		},
		{
			name:     "file path with line numbers",
			input:    "error in file.go:1234 and /path/to/source.cpp:56789",
			expected: "error in file.go:1234 and /path/to/source.cpp:56789",
		},
		{
			name:     "hex color codes and small error codes (should keep)",
			input:    "expected color 0xff0000ff, exit code 0x1, signal 0xb",
			expected: "expected color 0xff0000ff, exit code 0x1, signal 0xb",
		},
		{
			name:     "version numbers (should keep)",
			input:    "running glibc version 2.31.0.1 and go1.18.2.0 on host",
			expected: "running glibc version 2.31.0.1 and go1.18.2.0 on host",
		},
		{
			name:     "all combined",
			input:    "2026-08-19 19:20:13 Error on localhost:9000 (pid 1234): crash at 0x7fa2b8c9d0e1 in /b/s/w/ir/cache/work/skia/src/main.cpp",
			expected: "<DATE> <TIME> Error on localhost:<port> (pid <ID>): crash at 0x... in /<workdir>/skia/src/main.cpp",
		},
		{
			name: "Go stacktrace",
			input: `E0819 11:48:24.557086    2338 step.go:163] exit status 2
panic: exit status 2 [recovered]
	panic: exit status 2

goroutine 1 [running]:
go.skia.org/infra/task_driver/go/td.finishStep.deferwrap1()
	task_driver/go/td/step.go:127 +0x25
go.skia.org/infra/task_driver/go/td.finishStep({0x117a210, 0xc000503830}, {0xfbe4c0, 0xc002c20020})
	task_driver/go/td/step.go:130 +0x177
go.skia.org/infra/task_driver/go/td.EndRun({0x117a210, 0xc000503830})
	task_driver/go/td/run.go:241 +0xa5
panic({0xfbe4c0?, 0xc002c20020?})
	GOROOT/src/runtime/panic.go:792 +0x132
go.skia.org/infra/task_driver/go/td.Fatal({0x117a210, 0xc000503830}, {0x116bf40, 0xc002c20020})
	task_driver/go/td/step.go:167 +0xd9
main.main()
	infra/bots/task_drivers/command_wrapper/main.go:248 +0xfaf`,
			expected: `E<DATE> <TIME> <TID> step.go:163] exit status 2
panic: exit status 2 [recovered]
	panic: exit status 2

goroutine <ID> [running]:
go.skia.org/infra/task_driver/go/td.finishStep.deferwrap1()
	task_driver/go/td/step.go:127 +0x...
go.skia.org/infra/task_driver/go/td.finishStep({0x..., 0x...}, {0x..., 0x...})
	task_driver/go/td/step.go:130 +0x...
go.skia.org/infra/task_driver/go/td.EndRun({0x..., 0x...})
	task_driver/go/td/run.go:241 +0x...
panic({0x...?, 0x...?})
	GOROOT/src/runtime/panic.go:792 +0x...
go.skia.org/infra/task_driver/go/td.Fatal({0x..., 0x...}, {0x..., 0x...})
	task_driver/go/td/step.go:167 +0x...
main.main()
	infra/bots/task_drivers/command_wrapper/main.go:248 +0x...`,
		},
		{
			name:     "Windows Go log line",
			input:    "[D2026-08-19T09:17:59.928545-07:00 1300 0 system_windows.go:117] os.Interrrupt recieved, restoring signal handler.",
			expected: "[D<DATE>T<TIME> <PID> <TID> system_windows.go:117] os.Interrrupt recieved, restoring signal handler.",
		},
		{
			name: "failed vulkan call, no change",
			input: `Failed vulkan call. Error: -4, QueueSubmit(queue, 1, &submitInfo, fence)
Segmentation fault
+ >/data/local/tmp/rc
+ echo 139`,
			expected: `Failed vulkan call. Error: -4, QueueSubmit(queue, 1, &submitInfo, fence)
Segmentation fault
+ >/data/local/tmp/rc
+ echo 139`,
		},
		{
			name: "GrSurfaceTest failure with stacktrace",
			input: `FAILURE: ../../../../../skia/tests/GrSurfaceTest.cpp:329	Failed on ct kR_F16 format OpenGL-R16F 0xff0000ff is not 0x00000000 or 0xFF000000 [InitialTextureClear]

FAILURE: ../../../../../skia/tests/GrSurfaceTest.cpp:336	Failed on ct kRGBA_F16 format OpenGL-RGBA16F 0x00ff00ff != 0x00000000 [InitialTextureClear]

Command exited with code -11
#######################################
symbolized stacktrace follows
#######################################
build/dm crash_handler(int) at skia/dm/DM.cpp:407
/lib/x86_64-linux-gnu/libc.so.6 __restore_rt at libc_sigaction.c:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so trace_screen_create at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so vdp_imp_device_create_x11 at ??:?
build/dm GrGLFunction<void (unsigned int)>::operator()(unsigned int) const at skia/include/gpu/ganesh/gl/GrGLFunctions.h:?
build/dm GrGpu::regenerateMipMapLevels(GrTexture*) at skia/src/gpu/ganesh/GrGpu.cpp:649
build/dm resolve_and_mipmap(GrGpu*, GrSurfaceProxy*) at skia/src/gpu/ganesh/GrDrawingManager.cpp:515`,
			expected: `FAILURE: ../../../../../skia/tests/GrSurfaceTest.cpp:329	Failed on ct kR_F16 format OpenGL-R16F 0xff0000ff is not 0x00000000 or 0xFF000000 [InitialTextureClear]

FAILURE: ../../../../../skia/tests/GrSurfaceTest.cpp:336	Failed on ct kRGBA_F16 format OpenGL-RGBA16F 0x00ff00ff != 0x00000000 [InitialTextureClear]

Command exited with code -11
#######################################
symbolized stacktrace follows
#######################################
build/dm crash_handler(int) at skia/dm/DM.cpp:407
/lib/x86_64-linux-gnu/libc.so.6 __restore_rt at libc_sigaction.c:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so trace_screen_create at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so vdp_imp_device_create_x11 at ??:?
build/dm GrGLFunction<void (unsigned int)>::operator()(unsigned int) const at skia/include/gpu/ganesh/gl/GrGLFunctions.h:?
build/dm GrGpu::regenerateMipMapLevels(GrTexture*) at skia/src/gpu/ganesh/GrGpu.cpp:649
build/dm resolve_and_mipmap(GrGpu*, GrSurfaceProxy*) at skia/src/gpu/ganesh/GrDrawingManager.cpp:515`,
		},
		{
			name: "TSAN finding",
			input: `WARNING: ThreadSanitizer: data race (pid=36891)
  Write of size 8 at 0x722c00073ee0 by main thread:
    #0 free /tmp/clang/llvm-project/compiler-rt/lib/tsan/rtl/tsan_interceptors_posix.cpp:725:3 (dm+0x10fdf6f)
    #1 <null> <null> (libgallium-25.2.8-0ubuntu0.24.04.2.so+0x559e22) (BuildId: 6ccf17f6a28dda1c4c6e147c8d7503b974a501ba)
...
ThreadSanitizer: reported 1 warnings
Command exited with code 66`,
			expected: `WARNING: ThreadSanitizer: data race (pid=<ID>)
  Write of size 8 at 0x... by main thread:
    #0 free /<workdir>/llvm-project/compiler-rt/lib/tsan/rtl/tsan_interceptors_posix.cpp:725:3 (dm+0x...)
    #1 <null> <null> (libgallium-25.2.8-0ubuntu0.24.04.2.so+0x...) (BuildId: 6ccf17f6a28dda1c4c6e147c8d7503b974a501ba)
...
ThreadSanitizer: reported 1 warnings
Command exited with code 66`,
		},
		{
			name: "undefined behavior in downcast",
			input: `../../../../../skia/tests/graphite/StorageContextTest.cpp:103:24: runtime error: downcast of address 0x6060003d1ae0 which does not point to an object of type 'const SkGradientBaseShader'
0x6060003d1ae0: note: object is of type 'SkLocalMatrixShader'
 00 00 00 00  90 06 63 0a 01 00 00 00  01 00 00 00 00 00 80 3f  00 00 00 00 00 00 00 00  00 00 00 00
              ^~~~~~~~~~~~~~~~~~~~~^^
              vptr for 'SkLocalMatrixShader'
    #0 0x103d3dd5c in skgpu::graphite::test_StorageContextAppendVertexTest(skiatest::Reporter*, skgpu::graphite::Context*, skiatest::graphite::GraphiteTestContext*, skiatest::graphite::TestOptions const&)+0xce8 (dm:arm64+0x1017f5d5c)
    #1 0x1025808e4 in skiatest::graphite::RunWithGraphiteTestContexts(void (*)(skiatest::Reporter*, skgpu::graphite::Context*, skiatest::graphite::GraphiteTestContext*, skiatest::graphite::TestOptions const&), bool (*)(skgpu::ContextType), skiatest::Reporter*, skiatest::graphite::TestOptions const&)+0x1b0 (dm:arm64+0x1000388e4)
    #2 0x10430f778 in skiatest::Test::graphite(skiatest::Reporter*, skiatest::graphite::TestOptions const&) const+0x390 (dm:arm64+0x101dc7778)
    #3 0x102552bec in main+0x6260 (dm:arm64+0x10000abec)
    #4 0x183556b94  (<unknown module>)

SUMMARY: UndefinedBehaviorSanitizer: undefined-behavior ../../../../../skia/tests/graphite/StorageContextTest.cpp:103:24 in

Caught signal 6 [Abort trap: 6] (2700MB RAM, peak 2700MB), was running:
	unit test  StorageContextAppendVertexTest
Likely culprit:
	unit test  StorageContextAppendVertexTest`,
			expected: `../../../../../skia/tests/graphite/StorageContextTest.cpp:103:24: runtime error: downcast of address 0x... which does not point to an object of type 'const SkGradientBaseShader'
0x...: note: object is of type 'SkLocalMatrixShader'
 00 00 00 00  90 06 63 0a 01 00 00 00  01 00 00 00 00 00 80 3f  00 00 00 00 00 00 00 00  00 00 00 00
              ^~~~~~~~~~~~~~~~~~~~~^^
              vptr for 'SkLocalMatrixShader'
    #0 0x... in skgpu::graphite::test_StorageContextAppendVertexTest(skiatest::Reporter*, skgpu::graphite::Context*, skiatest::graphite::GraphiteTestContext*, skiatest::graphite::TestOptions const&)+0x... (dm:arm64+0x...)
    #1 0x... in skiatest::graphite::RunWithGraphiteTestContexts(void (*)(skiatest::Reporter*, skgpu::graphite::Context*, skiatest::graphite::GraphiteTestContext*, skiatest::graphite::TestOptions const&), bool (*)(skgpu::ContextType), skiatest::Reporter*, skiatest::graphite::TestOptions const&)+0x... (dm:arm64+0x...)
    #2 0x... in skiatest::Test::graphite(skiatest::Reporter*, skiatest::graphite::TestOptions const&) const+0x... (dm:arm64+0x...)
    #3 0x... in main+0x... (dm:arm64+0x...)
    #4 0x...  (<unknown module>)

SUMMARY: UndefinedBehaviorSanitizer: undefined-behavior ../../../../../skia/tests/graphite/StorageContextTest.cpp:103:24 in

Caught signal 6 [Abort trap: 6] (2700MB RAM, peak 2700MB), was running:
	unit test  StorageContextAppendVertexTest
Likely culprit:
	unit test  StorageContextAppendVertexTest`,
		},
		{
			name: "segfault in nanobench (libgallium)",
			input: `Signal 11 [Segmentation fault]:
    /b/s/w/ir/build/nanobench(+0x10ff168) [0x5ac6c3655168]
    /lib/x86_64-linux-gnu/libc.so.6(+0x45330) [0x7fb766c45330]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x116a166) [0x7fb75ff6a166]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x116ca11) [0x7fb75ff6ca11]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x116cbef) [0x7fb75ff6cbef]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x10f14b5) [0x7fb75fef14b5]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x10f4a4b) [0x7fb75fef4a4b]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x423c88) [0x7fb75f223c88]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x4246ad) [0x7fb75f2246ad]
    /b/s/w/ir/build/nanobench(+0x1416f48) [0x5ac6c396cf48]
    /b/s/w/ir/build/nanobench(+0x1423aff) [0x5ac6c3979aff]
    /b/s/w/ir/build/nanobench(+0x143e190) [0x5ac6c3994190]
    /b/s/w/ir/build/nanobench(+0x133750a) [0x5ac6c388d50a]
    /b/s/w/ir/build/nanobench(+0x13dced0) [0x5ac6c3932ed0]
    /b/s/w/ir/build/nanobench(+0x1320beb) [0x5ac6c3876beb]
    /b/s/w/ir/build/nanobench(+0x13205d0) [0x5ac6c38765d0]
    /b/s/w/ir/build/nanobench(+0x132119f) [0x5ac6c387719f]
    /b/s/w/ir/build/nanobench(+0x10f2d1f) [0x5ac6c3648d1f]
    /b/s/w/ir/build/nanobench(+0xef02ab) [0x5ac6c34462ab]
    /b/s/w/ir/build/nanobench(+0xee5b42) [0x5ac6c343bb42]
    /lib/x86_64-linux-gnu/libc.so.6(+0x2a1ca) [0x7fb766c2a1ca]
    /lib/x86_64-linux-gnu/libc.so.6(__libc_start_main+0x8b) [0x7fb766c2a28b]
    /b/s/w/ir/build/nanobench(_start+0x25) [0x5ac6c343a245]
Command exited with code 11
#######################################
symbolized stacktrace follows
#######################################
build/nanobench handler(int) at skia/tools/CrashHandler.cpp:96
/lib/x86_64-linux-gnu/libc.so.6 __restore_rt at libc_sigaction.c:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so vdp_imp_device_create_x11 at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so vdp_imp_device_create_x11 at ??:?
build/nanobench GrGLFunction<void (int, int, int, int, int, int, int, int, unsigned int, unsigned int)>::GrGLFunction(void (*)(int, int, int, int, int, int, int, int, unsigned int, unsigned int))::{lambda(void const*, int, int, int, int, int, int, int, int, unsigned int, unsigned int)#1}::operator()(void const*, int, int, int, int, int, int, int, int, unsigned int, unsigned int) const at skia/include/gpu/ganesh/gl/GrGLFunctions.h:277
build/nanobench GrGLFunction<void (int, int, int, int, int, int, int, int, unsigned int, unsigned int)>::operator()(int, int, int, int, int, int, int, int, unsigned int, unsigned int) const at skia/include/gpu/ganesh/gl/GrGLFunctions.h:300
build/nanobench GrGLOpsRenderPass::onEnd() at skia/src/gpu/ganesh/gl/GrGLOpsRenderPass.cpp:93
build/nanobench sk_sp<GrBuffer const>::reset(GrBuffer const*) at skia/include/core/SkRefCnt.h:314
build/nanobench GrOpFlushState::gpu() at skia/src/gpu/ganesh/GrOpFlushState.h:85
build/nanobench GrRenderTask::execute(GrOpFlushState*) at skia/src/gpu/ganesh/GrRenderTask.h:53
build/nanobench GrDrawingManager::flush(SkSpan<GrSurfaceProxy*>, SkSurfaces::BackendSurfaceAccess, GrFlushInfo const&, skgpu::MutableTextureState const*) at skia/src/gpu/ganesh/GrDrawingManager.cpp:212
build/nanobench GrDrawingManager::flushSurfaces(SkSpan<GrSurfaceProxy*>, SkSurfaces::BackendSurfaceAccess, GrFlushInfo const&, skgpu::MutableTextureState const*) at skia/src/gpu/ganesh/GrDrawingManager.cpp:547
build/nanobench GrSubmitInfo::GrSubmitInfo() at skia/include/gpu/ganesh/GrTypes.h:190
build/nanobench SkAutoCanvasRestore::SkAutoCanvasRestore(SkCanvas*, bool) at skia/include/core/SkCanvas.h:2732
build/nanobench std::_Function_base::~_Function_base() at include/c++/13/bits/std_function.h:243
/lib/x86_64-linux-gnu/libc.so.6 __libc_start_call_main at .sysdeps/x86/libc-start.c:74
/lib/x86_64-linux-gnu/libc.so.6(__libc_start_main+0x8b) [0x7fb766c2a28b]
/<workdir>/nanobench(_start+0x25) [0x5ac6c343a245]`,
			expected: `Signal 11 [Segmentation fault]:
    /<workdir>/nanobench(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libc.so.6(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /<workdir>/nanobench(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libc.so.6(+0x...) [0x...]
    /lib/x86_64-linux-gnu/libc.so.6(__libc_start_main+0x...) [0x...]
    /<workdir>/nanobench(_start+0x...) [0x...]
Command exited with code 11
#######################################
symbolized stacktrace follows
#######################################
build/nanobench handler(int) at skia/tools/CrashHandler.cpp:96
/lib/x86_64-linux-gnu/libc.so.6 __restore_rt at libc_sigaction.c:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so std::vector<unsigned int, std::allocator<unsigned int> >::_M_default_append(unsigned long) at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so vdp_imp_device_create_x11 at ??:?
/lib/x86_64-linux-gnu/libgallium-25.2.8-0ubuntu0.24.04.2.so vdp_imp_device_create_x11 at ??:?
build/nanobench GrGLFunction<void (int, int, int, int, int, int, int, int, unsigned int, unsigned int)>::GrGLFunction(void (*)(int, int, int, int, int, int, int, int, unsigned int, unsigned int))::{lambda(void const*, int, int, int, int, int, int, int, int, unsigned int, unsigned int)#1}::operator()(void const*, int, int, int, int, int, int, int, int, unsigned int, unsigned int) const at skia/include/gpu/ganesh/gl/GrGLFunctions.h:277
build/nanobench GrGLFunction<void (int, int, int, int, int, int, int, int, unsigned int, unsigned int)>::operator()(int, int, int, int, int, int, int, int, unsigned int, unsigned int) const at skia/include/gpu/ganesh/gl/GrGLFunctions.h:300
build/nanobench GrGLOpsRenderPass::onEnd() at skia/src/gpu/ganesh/gl/GrGLOpsRenderPass.cpp:93
build/nanobench sk_sp<GrBuffer const>::reset(GrBuffer const*) at skia/include/core/SkRefCnt.h:314
build/nanobench GrOpFlushState::gpu() at skia/src/gpu/ganesh/GrOpFlushState.h:85
build/nanobench GrRenderTask::execute(GrOpFlushState*) at skia/src/gpu/ganesh/GrRenderTask.h:53
build/nanobench GrDrawingManager::flush(SkSpan<GrSurfaceProxy*>, SkSurfaces::BackendSurfaceAccess, GrFlushInfo const&, skgpu::MutableTextureState const*) at skia/src/gpu/ganesh/GrDrawingManager.cpp:212
build/nanobench GrDrawingManager::flushSurfaces(SkSpan<GrSurfaceProxy*>, SkSurfaces::BackendSurfaceAccess, GrFlushInfo const&, skgpu::MutableTextureState const*) at skia/src/gpu/ganesh/GrDrawingManager.cpp:547
build/nanobench GrSubmitInfo::GrSubmitInfo() at skia/include/gpu/ganesh/GrTypes.h:190
build/nanobench SkAutoCanvasRestore::SkAutoCanvasRestore(SkCanvas*, bool) at skia/include/core/SkCanvas.h:2732
build/nanobench std::_Function_base::~_Function_base() at include/c++/13/bits/std_function.h:243
/lib/x86_64-linux-gnu/libc.so.6 __libc_start_call_main at .sysdeps/x86/libc-start.c:74
/lib/x86_64-linux-gnu/libc.so.6(__libc_start_main+0x...) [0x...]
/<workdir>/nanobench(_start+0x...) [0x...]`,
		},
		{
			name: "strip failures text",
			input: `Failures:
        ../../../../../skia/tests/GrClipStackTest.cpp:2234      Failed to read pixels [ClipStack_MixedAA, Vulkan]: surface->readPixels(dstColumn, 15, 0)
        ../../../../../skia/tests/GrClipStackTest.cpp:2245      AA draw leaked beyond non-AA clip [ClipStack_MixedAA, Vulkan]: !leak
2 failures`,
			expected: `../../../../../skia/tests/GrClipStackTest.cpp:2234      Failed to read pixels [ClipStack_MixedAA, Vulkan]: surface->readPixels(dstColumn, 15, 0)
../../../../../skia/tests/GrClipStackTest.cpp:2245      AA draw leaked beyond non-AA clip [ClipStack_MixedAA, Vulkan]: !leak`,
		},
		{
			name: "strip stdout+stderr",
			input: `Command exited with exit status 0xc0000135: ... bazelisk build //tools:full_build --config=for_windows_x64_release --experimental_scale_timeouts=2.0; Stdout+Stderr:
blah
blah`,
			expected: `Command exited with exit status 0xc0000135: ... bazelisk build //tools:full_build --config=for_windows_x64_release --experimental_scale_timeouts=2.0
blah
blah`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := SanitizeErrorText(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}

func TestDedent(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
		{
			name:     "no indent",
			input:    "line 1\nline 2",
			expected: "line 1\nline 2",
		},
		{
			name:     "common space indent",
			input:    "  line 1\n  line 2",
			expected: "line 1\nline 2",
		},
		{
			name:     "differing space indent",
			input:    "  line 1\n    line 2",
			expected: "line 1\n  line 2",
		},
		{
			name:     "common tab indent",
			input:    "\tline 1\n\tline 2",
			expected: "line 1\nline 2",
		},
		{
			name:     "differing tab indent",
			input:    "\tline 1\n\t\tline 2",
			expected: "line 1\n\tline 2",
		},
		{
			name:     "mixed spaces and tabs - no common prefix",
			input:    "  line 1\n\tline 2",
			expected: "  line 1\n\tline 2",
		},
		{
			name:     "empty lines are ignored for determining common indent but trimmed",
			input:    "  \n\n  line 1\n    line 2\n",
			expected: "\n\nline 1\n  line 2\n",
		},
		{
			name: "real example with empty lines",
			input: `
    func hello() {
        fmt.Println("hello")
    }
`,
			expected: `
func hello() {
    fmt.Println("hello")
}
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			actual := dedent(tt.input)
			assert.Equal(t, tt.expected, actual)
		})
	}
}
