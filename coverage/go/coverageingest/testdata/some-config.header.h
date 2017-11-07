Coverage Report
Created: 2017-10-26 17:47
/mnt/pd0/work/skia/dm/DM.h:
    1|       |// Based off of real world DM data
    2|       |
    3|       |#include "DMJsonWriter.h"
    4|       |class Error {
    5|       |public:
    6|      0|    Error(const SkString& s) : fMsg(s), fFatal(!this->isEmpty()) {}
    7|     17|    Error(const char* s)     : fMsg(s), fFatal(!this->isEmpty()) {}
    8|       |
    9|       |    Error(const Error&)            = default;
   10|     10|    Error& operator=(const Error&) = default;