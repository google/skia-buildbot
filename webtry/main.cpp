#include <sys/time.h>
#include <sys/resource.h>
#include <fcntl.h>

#include "GrContextFactory.h"

#include "SkBase64.h"
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

static void dumpOutput(SkDynamicMemoryWStream *stream, const char *name, bool last=true) {
    SkAutoDataUnref pngData(stream->copyToData());
    size_t b64Size = SkBase64::Encode(pngData->data(), pngData->size(), NULL);
    SkAutoTMalloc<char> b64Data(b64Size);
    SkBase64::Encode(pngData->data(), pngData->size(), b64Data.get());
    printf( "\t\"%s\": \"%.*s\"", name, (int) b64Size, b64Data.get() );
    if (!last) {
        printf( "," );
    }
    printf( "\n" );
}

int main(int argc, char** argv) {
    SkCommandLineFlags::Parse(argc, argv);
    SkAutoGraphics init;

    if (FLAGS_source.count() == 1) {
        const char *sourceDir = getenv("WEBTRY_INOUT");
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

    SkDynamicMemoryWStream* streams[3] = {NULL, NULL, NULL};

    if (FLAGS_raster) {
        streams[0] = SkNEW(SkDynamicMemoryWStream);
    }
    if (FLAGS_gpu) {
        streams[1] = SkNEW(SkDynamicMemoryWStream);
    }
    if (FLAGS_pdf) {
        streams[2] = SkNEW(SkDynamicMemoryWStream);
    }

    SkImageInfo info = SkImageInfo::MakeN32(FLAGS_width, FLAGS_height, kPremul_SkAlphaType);

    GrContext *gr = NULL;
    GrContextFactory* grFactory = NULL;

    // need to set up the GPU context before we install system call restrictions
    if (FLAGS_gpu) {
        GrContextOptions grContextOpts;
        grFactory = new GrContextFactory(grContextOpts);
        gr = grFactory->get(GrContextFactory::kMESA_GLContextType);
    }

    printf( "{\n" );

    if (NULL != streams[0]) {
        drawRaster(streams[0], info);
        dumpOutput(streams[0], "Raster", NULL == streams[1] && NULL == streams[2] );
    }
    if (NULL != streams[1]) {
        drawGPU(streams[1], gr, info);
        dumpOutput(streams[1], "Gpu", NULL == streams[2] );
    }
    if (NULL != streams[2]) {
        drawPDF(streams[2], info);
        dumpOutput(streams[2], "Pdf" );
    }

    printf( "}\n" );

    if (gr) {
        delete grFactory;
    }
}
