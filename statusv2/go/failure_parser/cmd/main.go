package main

import (
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/statusv2/go/failure_parser"
	"go.skia.org/infra/task_scheduler/go/db/remote_db"
)

var (
	// Flags.
	taskSchedulerDbUrl = flag.String("task_db_url", "http://skia-task-scheduler:8008/db/", "Where the Skia task scheduler database is hosted.")
)

func main() {
	common.Init()

	///*
	db, err := remote_db.NewClient(*taskSchedulerDbUrl)
	if err != nil {
		sklog.Fatal(err)
	}

	fp, err := failure_parser.New(db)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := fp.Tick(); err != nil {
		sklog.Fatal(err)
	}
	//*/

	/*
			failures := failure_parser.ParseFailures(`[716/1974] compile ../../../src/gpu/glsl/GrGLSLVertexShaderBuilder.cpp
		[717/1974] compile ../../../src/gpu/glsl/GrGLSLVarying.cpp
		[718/1974] compile ../../../src/gpu/SkGpuDevice_drawTexture.cpp
		[719/1974] compile ../../../src/gpu/GrRenderTargetContext.cpp
		[720/1974] compile ../../../src/gpu/gl/GrGLVertexArray.cpp
		[721/1974] compile ../../../src/gpu/SkGr.cpp
		[722/1974] compile ../../../src/sksl/SkSLHCodeGenerator.cpp
		FAILED: obj/src/sksl/gpu.SkSLHCodeGenerator.obj
		c:\b\s\w\ir\t\depot_tools\win_toolchain\vs_files\d3cb0e37bdd120ad0ac4650b674b09e81be45616/VC/bin/amd64/cl.exe /nologo /showIncludes /FC @obj/src/sksl/gpu.SkSLHCodeGenerator.obj.rsp /c ../../../src/sksl/SkSLHCodeGenerator.cpp /Foobj/src/sksl/gpu.SkSLHCodeGenerator.obj /Fd"obj/gpu_c.pdb"
		c:\b\work\skia\src\sksl\skslhcodegenerator.h(29): error C3861: 'toupper': identifier not found
		[723/1974] compile ../../../src/sksl/SkSLCompiler.cpp
		FAILED: obj/src/sksl/gpu.SkSLCompiler.obj
		c:\b\s\w\ir\t\depot_tools\win_toolchain\vs_files\d3cb0e37bdd120ad0ac4650b674b09e81be45616/VC/bin/amd64/cl.exe /nologo /showIncludes /FC @obj/src/sksl/gpu.SkSLCompiler.obj.rsp /c ../../../src/sksl/SkSLCompiler.cpp /Foobj/src/sksl/gpu.SkSLCompiler.obj /Fd"obj/gpu_c.pdb"
		c:\b\work\skia\src\sksl\skslhcodegenerator.h(29): error C3861: 'toupper': identifier not found
		[724/1974] compile ../../../src/sksl/SkSLCPPCodeGenerator.cpp
		FAILED: obj/src/sksl/gpu.SkSLCPPCodeGenerator.obj
		c:\b\s\w\ir\t\depot_tools\win_toolchain\vs_files\d3cb0e37bdd120ad0ac4650b674b09e81be45616/VC/bin/amd64/cl.exe /nologo /showIncludes /FC @obj/src/sksl/gpu.SkSLCPPCodeGenerator.obj.rsp /c ../../../src/sksl/SkSLCPPCodeGenerator.cpp /Foobj/src/sksl/gpu.SkSLCPPCodeGenerator.obj /Fd"obj/gpu_c.pdb"
		c:\b\work\skia\src\sksl\skslhcodegenerator.h(29): error C3861: 'toupper': identifier not found
		[725/1974] compile ../../../src/gpu/gl/builders/GrGLShaderStringBuilder.cpp
		[726/1974] compile ../../../src/gpu/SkGpuDevice.cpp
		[727/1974] compile ../../../src/image/SkImage_Gpu.cpp
		[728/1974] compile ../../../src/gpu/gl/builders/GrGLProgramBuilder.cpp
		[729/1974] compile ../../../src/sksl/SkSLUtil.cpp
		[730/1974] compile ../../../src/sksl/lex.layout.cpp
		[731/1974] compile ../../../src/image/SkSurface_Gpu.cpp
		[732/1974] compile ../../../src/gpu/glsl/GrGLSLFragmentShaderBuilder.cpp
		[733/1974] compile ../../../src/sksl/ir/SkSLSymbolTable.cpp
		[734/1974] compile ../../../src/sksl/SkSLString.cpp
		[735/1974] compile ../../../src/sksl/SkSLCFGGenerator.cpp
		[736/1974] compile ../../../src/sksl/SkSLGLSLCodeGenerator.cpp
		[737/1974] compile ../../../src/sksl/SkSLParser.cpp
		[738/1974] compile ../../../src/sksl/SkSLIRGenerator.cpp`)
			if len(failures) == 0 {
				sklog.Fatal("no failures")
			}
			for _, f := range failures {
				sklog.Info(f.StrippedMessage)
			}
			//*/
}
