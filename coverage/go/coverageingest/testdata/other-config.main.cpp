Coverage Report
Created: 2017-10-26 17:47
/mnt/pd0/work/skia/dm/DM.cpp:
    1|       |// Based off of real world DM data
    2|       |
    3|       |#include "DMJsonWriter.h"
    4|       |#include "DMSrcSink.h"
    5|       |template <typename... Args>
    6|   149k|static void vlog(const char* fmt, Args&&... args) {
    7|   149k|    if (gVLog) {
    8|   149k|        char s[64];
    9|   149k|        HumanizeMs(s, 64, SkTime::GetMSecs() - kStartMs);
   10|   149k|        fprintf(gVLog, "%s\t", s);
   11|   149k|        fprintf(gVLog, fmt, args...);
   12|   149k|        fflush(gVLog);
   13|   149k|    }
   14|   149k|}
   ------------------
  | Unexecuted instantiation: DM.cpp:_ZL4vlogIJRiRPcEEvPKcDpOT_
  ------------------
  | Unexecuted instantiation: DM.cpp:_ZL4vlogIJRPcEEvPKcDpOT_
  ------------------
  | Unexecuted instantiation: DM.cpp:_ZL4vlogIJRiS0_EEvPKcDpOT_
  ------------------
  | Unexecuted instantiation: DM.cpp:_ZL4vlogIJRPKcS2_EEvS1_DpOT_
  ------------------
  | DM.cpp:_ZL4vlogIJPKcEEvS1_DpOT_:
  |    6|   146k|static void vlog(const char* fmt, Args&&... args) {
  |    7|   146k|    if (gVLog) {
  |    8|   146k|        char s[64];
  |    9|   146k|        HumanizeMs(s, 64, SkTime::GetMSecs() - kStartMs);
  |   10|   146k|        fprintf(gVLog, "%s\t", s);
  |   11|   146k|        fprintf(gVLog, fmt, args...);
  |   12|   146k|        fflush(gVLog);
  |   13|   146k|    }
  |   14|   146k|}
   15|      0|static void fail(const SkString& err) {
   16|      0|    SkAutoMutexAcquire lock(gFailuresMutex);
   17|      0|    SkDebugf("\n\nFAILURE: %s\n\n", err.c_str());
   18|      0|    gFailures.push_back(err);
   19|      0|}
   20|       |
   21|       |struct Running {
   22|       |    SkString   id;
   23|       |    SkThreadID thread;
   24|       |
   25|  2.25k|    void dump() const {
   26|  2.25k|        info("\t%s\n", id.c_str());
   27|  2.25k|    }
   28|       |};
   29|       |    #if !defined(SK_BUILD_FOR_ANDROID)
   30|       |        void* stack[64];
   31|       |        int count = backtrace(stack, SK_ARRAY_COUNT(stack));
   32|       |        char** symbols = backtrace_symbols(stack, count);
   33|       |        info("\nStack trace:\n");
   34|       |        for (int i = 0; i < count; i++) {
   35|       |            info("    %s\n", symbols[i]);
   36|       |        }
   37|       |    #else
   38|     19|        fflush(stdout);
   39|      0|    #endif
   40|      0|        signal(sig, previous_handler[sig]);
   41|      0|        raise(sig);
   42|      0|    }