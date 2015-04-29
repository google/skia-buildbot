#include <sys/time.h>
#include <sys/resource.h>

#include "GrContextFactory.h"

#include "SkCanvas.h"
#include "SkCommandLineFlags.h"
#include "SkData.h"
#include "SkDocument.h"
#include "SkFontMgr.h"
#include "SkForceLinking.h"
#include "SkGraphics.h"
#include "SkImageDecoder.h"
#include "SkImageEncoder.h"
#include "SkImageInfo.h"
#include "SkOSFile.h"
#include "SkStream.h"
#include "SkSurface.h"

__SK_FORCE_IMAGE_DECODER_LINKING;

DEFINE_string(out, "", "Output basename; fuzzer will append the config used and the appropriate extension");
DEFINE_string(source, "", "Filename of the source image.");
DEFINE_int32(width, 256, "Width of output image.");
DEFINE_int32(height, 256, "Height of output image.");
DEFINE_bool(gpu, false, "Use GPU (Mesa) rendering.");
DEFINE_bool(raster, true, "Use Raster rendering.");
DEFINE_bool(pdf, false, "Use PDF rendering.");

// Defined in template.cpp.
extern SkBitmap source;

extern void draw(SkCanvas* canvas);

static void drawAndDump(SkSurface* surface, SkWStream* stream) {
    SkCanvas *canvas = surface->getCanvas();
    draw(canvas);

    // Write out the image as a PNG.
    SkAutoTUnref<SkImage> image(surface->newImageSnapshot());
    SkAutoTUnref<SkData> data(image->encode(SkImageEncoder::kPNG_Type, 100));
    if (NULL == data.get()) {
        printf("Failed to encode\n");
        exit(1);
    }
    stream->write(data->data(), data->size());
}

static void drawRaster(SkWStream* stream, SkImageInfo info) {
    SkAutoTUnref<SkSurface> surface;
    surface.reset(SkSurface::NewRaster(info));
    drawAndDump(surface, stream);
}

static void drawGPU(SkWStream* stream, GrContext* gr, SkImageInfo info) {
    SkAutoTUnref<SkSurface> surface;
    surface.reset(SkSurface::NewRenderTarget(gr,SkSurface::kNo_Budgeted,info));

    drawAndDump(surface, stream);
}

static void drawPDF(SkWStream* stream, SkImageInfo info) {
    SkAutoTUnref<SkDocument> document(SkDocument::CreatePDF(stream));
    SkCanvas *canvas = document->beginPage(info.width(), info.height());

    SkAutoTDelete<SkStreamAsset> pdfData;

    draw(canvas);

    canvas->flush();
    document->endPage();
    document->close();
}

int main(int argc, char** argv) {
    SkCommandLineFlags::Parse(argc, argv);
    SkAutoGraphics init;

    if (FLAGS_out.count() == 0) {
      perror("The --out flag must have an argument.");
      return 1;
    }

    if (FLAGS_source.count() == 1) {
        const char *sourceDir = getenv("FUZZER_INOUT");
        if (NULL == sourceDir) {
            sourceDir = "/skia_build/inout";
        }

        SkString sourcePath = SkOSPath::Join(sourceDir, FLAGS_source[0]);
        if (!SkImageDecoder::DecodeFile(sourcePath.c_str(), &source)) {
            perror("Unable to read the source image.");
        }
    }

    // make sure to open any needed output files before we set up the security
    // jail

    SkWStream* streams[3] = {NULL, NULL, NULL};

    if (FLAGS_raster) {
        SkString outPath;
        outPath.printf("%s_raster.png", FLAGS_out[0]);
        streams[0] = SkNEW_ARGS(SkFILEWStream,(outPath.c_str()));
    }
    if (FLAGS_gpu) {
        SkString outPath;
        outPath.printf("%s_gpu.png", FLAGS_out[0]);
        streams[1] = SkNEW_ARGS(SkFILEWStream,(outPath.c_str()));
    }
    if (FLAGS_pdf) {
        SkString outPath;
        outPath.printf("%s.pdf", FLAGS_out[0]);
        streams[2] = SkNEW_ARGS(SkFILEWStream,(outPath.c_str()));
    }

    SkImageInfo info = SkImageInfo::MakeN32(FLAGS_width, FLAGS_height, kPremul_SkAlphaType);

    GrContext *gr = NULL;
    GrContextFactory* grFactory = NULL;

    // need to set up the GPU context before we install system call restrictions
    if (FLAGS_gpu) {
        GrContext::Options grContextOpts;
        grFactory = new GrContextFactory(grContextOpts);
        gr = grFactory->get(GrContextFactory::kMESA_GLContextType);
    }

    // RefDefault will cause the custom font manager to scan the system for fonts
    // and cache an SkStream for each one; that way we don't have to open font files
    // after we've set up the chroot jail.

    SkAutoTUnref<SkFontMgr> unusedFM(SkFontMgr::RefDefault());

    if (NULL != streams[0]) {
        drawRaster(streams[0], info);
    }
    if (NULL != streams[1]) {
        drawGPU(streams[1], gr, info);
    }
    if (NULL != streams[2]) {
        drawPDF(streams[2], info);
    }

    if (gr) {
        delete grFactory;
    }
}
