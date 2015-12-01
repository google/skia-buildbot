package functionnamefinder

import (
	"bufio"
	"bytes"
	"reflect"
	"testing"

	"go.skia.org/infra/fuzzer/go/common"
)

func TestFindMethodDefs(t *testing.T) {
	// The real ast output has each file multiple times.
	// The implementation should handle this properly.
	b := bytes.NewBuffer([]byte(SAMPLE_AST + SAMPLE_AST + SAMPLE_AST))
	br := bufio.NewScanner(b)
	methodDefs, err := parseSkiaAST(br)
	if err != nil {
		t.Errorf("Problem parsing skia ast: %s", err)
	}

	if !reflect.DeepEqual(mockMethodMap, methodDefs) {
		t.Errorf("%#v\n expected but was %#v", mockMethodMap, methodDefs)
	}
}

func TestFunctionName(t *testing.T) {
	if name := mockMethodMap.FunctionName("doesnt", "exist", -1); name != common.UNKNOWN_FUNCTION {
		t.Errorf("Expected %s but was %s", common.UNKNOWN_FUNCTION, name)
	}
	if name := mockMethodMap.FunctionName("include/core/", "SkPixelSerializer.h", 0); name != common.UNKNOWN_FUNCTION {
		t.Errorf("Expected %s but was %s", common.UNKNOWN_FUNCTION, name)
	}
	if name := mockMethodMap.FunctionName("include/core/", "SkPixelSerializer.h", 19); name != "SkPixelSerializer(SkPixelSerializer &)" {
		t.Errorf("Expected %s but was %s", "SkPixelSerializer(SkPixelSerializer &)", name)
	}
	if name := mockMethodMap.FunctionName("include/core/", "SkPixelSerializer.h", 20); name != "SkPixelSerializer(SkPixelSerializer &)" {
		t.Errorf("Expected %s but was %s", "SkPixelSerializer(SkPixelSerializer &)", name)
	}
	if name := mockMethodMap.FunctionName("include/core/", "SkPixelSerializer.h", 26); name != "SkPixelSerializer(SkPixelSerializer &)" {
		t.Errorf("Expected %s but was %s", "SkPixelSerializer(SkPixelSerializer &)", name)
	}
	if name := mockMethodMap.FunctionName("include/core/", "SkPixelSerializer.h", 27); name != "useEncodedData(void *, size_t)" {
		t.Errorf("Expected %s but was %s", "useEncodedData(void *, size_t)", name)
	}
	if name := mockMethodMap.FunctionName("include/core/", "SkPatch3D.cpp", 9999999); name != "SkAutoSMalloc<kSize>(void)" {
		t.Errorf("Expected %s but was %s", "SkAutoSMalloc<kSize>(void)", name)
	}
}

var mockMethodMap = methodLookupMap{
	file{"include/core/", "SkPixelSerializer.h"}: {
		{
			StartLine: 19,
			Name:      "SkPixelSerializer(SkPixelSerializer &)",
		},
		{
			StartLine: 27,
			Name:      "useEncodedData(void *, size_t)",
		},
		{
			StartLine: 35,
			Name:      "encodePixels(SkImageInfo &, void *, size_t)",
		},
		{
			StartLine: 44,
			Name:      "onUseEncodedData(void *, size_t)",
		},
		{
			StartLine: 50,
			Name:      "onEncodePixels(void)",
		},
		{
			StartLine: 107,
			Name:      "asABlurShadow(SkDrawLooper::BlurShadowRec *)",
		},
	},
	file{"include/core/", "SkPatch3D.cpp"}: {
		{
			StartLine: 75,
			Name:      "SkPatch3D(void)",
		},
		{
			StartLine: 77,
			Name:      "reset(void)",
		},
		{
			StartLine: 78,
			Name:      "transform(SkMatrix3D &, SkPatch3D *)",
		},
		{
			StartLine: 82,
			Name:      "DotWith(SkVector3D &)",
		},
		{
			StartLine: 94,
			Name:      "operator-(void)",
		},
		{
			StartLine: 591,
			Name:      "SkAutoSMalloc<kSize>(void)",
		},
	},
}

// A relatively realistic version of a Clang AST, execpt with ' replaced with ', for stringability reasons
const SAMPLE_AST = `|-CXXRecordDecl 0x304fcd0 prev 0x2f34d70 <../../include/core/SkPixelSerializer.h:13:1, col:7> col:7 referenced class SkData
|-CXXRecordDecl 0x304fd60 prev 0x2b337f0 <line:14:1, col:8> col:8 referenced struct SkImageInfo
|-CXXRecordDecl 0x304fe20 prev 0x3004810 <line:19:1, line:51:1> line:19:7 referenced class SkPixelSerializer definition
| |-public 'class SkRefCnt'
| |-FullComment 0x3818700 <line:17:3, col:69>
| | '-ParagraphComment 0x38186d0 <col:3, col:69>
| |   '-TextComment 0x38186a0 <col:3, col:69> Text="  Interface for serializing pixels, e.g. SkBitmaps in an SkPicture."
| |-CXXRecordDecl 0x304ff50 <line:19:1, col:7> col:7 implicit referenced class SkPixelSerializer
| |-CXXRecordDecl 0x23dca40 <line:91:5, line:97:5> line:91:12 referenced struct BlurShadowRec definition
| | |-CXXRecordDecl 0x23dcb50 <col:5, col:12> col:12 implicit struct BlurShadowRec
| | |-FieldDecl 0x23dcbf0 <line:92:9, col:25> col:25 fSigma 'SkScalar':'float'
| | |-FieldDecl 0x23dcc50 <line:93:9, col:25> col:25 fOffset 'SkVector':'struct SkPoint'
| | |-FieldDecl 0x23dccb0 <line:94:9, col:25> col:25 fColor 'SkColor':'unsigned int'
| | |-FieldDecl 0x23dcd10 <line:95:9, col:25> col:25 fStyle 'enum SkBlurStyle'
| | |-FieldDecl 0x23dcd70 <line:96:9, col:25> col:25 fQuality 'enum SkBlurQuality'
| | '-CXXMethodDecl 0x23dcef0 <line:107:5, col:48> col:18 asABlurShadow '_Bool (struct SkDrawLooper::BlurShadowRec *) const' virtual
| | | |-ParmVarDecl 0x23dce30 <col:32, col:45> col:46 'struct SkDrawLooper::BlurShadowRec *'
| | '-FullComment 0x24b7050 <line:99:7, line:105:93>
| |   |-ParagraphComment 0x24b6fd0 <line:99:7, line:103:71>
| |   | |-TextComment 0x24b6f00 <line:99:7, col:73> Text="  If this looper can be interpreted as having two layers, such that"
| |   | |-TextComment 0x24b6f20 <line:100:7, col:74> Text="      1. The first layer (bottom most) just has a blur and translate"
| |   | |-TextComment 0x24b6f40 <line:101:7, col:78> Text="      2. The second layer has no modifications to either paint or canvas"
| |   | |-TextComment 0x24b6f60 <line:102:7, col:31> Text="      3. No other layers."
| |   | '-TextComment 0x24b6f80 <line:103:7, col:71> Text="  then return true, and if not null, fill out the BlurShadowRec)."
| |-AccessSpecDecl 0x304ffe0 <line:20:1, col:7> col:1 public
| |-CXXDestructorDecl 0x3050060 <line:21:5, col:35> col:13 used ~SkPixelSerializer 'void (void) noexcept' virtual
| | '-CompoundStmt 0x3050f40 <col:34, col:35>
| |-CXXMethodDecl 0x30502d0 <line:27:5, line:29:5> line:27:10 useEncodedData '_Bool (const void *, size_t)'
| | |-ParmVarDecl 0x3050150 <col:25, col:37> col:37 used data 'const void *'
| | |-ParmVarDecl 0x30501c0 <col:43, col:50> col:50 used len 'size_t':'unsigned long'
| | |-CompoundStmt 0x3051078 <col:55, line:29:5>
| | | '-ReturnStmt 0x3051058 <line:28:9, col:48>
| | |   '-CXXMemberCallExpr 0x3050ff0 <col:16, col:48> '_Bool'
| | |     |-MemberExpr 0x3050f70 <col:16, col:22> '<bound member method type>' ->onUseEncodedData 0x3050800
| | |     | '-CXXThisExpr 0x3050f58 <col:16> 'class SkPixelSerializer *' this
| | |     |-ImplicitCastExpr 0x3051028 <col:39> 'const void *' <LValueToRValue>
| | |     | '-DeclRefExpr 0x3050fa0 <col:39> 'const void *' lvalue ParmVar 0x3050150 'data' 'const void *'
| | |     '-ImplicitCastExpr 0x3051040 <col:45> 'size_t':'unsigned long' <LValueToRValue>
| | |       '-DeclRefExpr 0x3050fc8 <col:45> 'size_t':'unsigned long' lvalue ParmVar 0x30501c0 'len' 'size_t':'unsigned long'
| | '-FullComment 0x38187f0 <line:24:7, line:25:75>
| |   '-ParagraphComment 0x38187c0 <line:24:7, line:25:75>
| |     |-TextComment 0x3818770 <line:24:7, col:79> Text="  Call to determine if the client wants to serialize the encoded data. If"
| |     '-TextComment 0x3818790 <line:25:7, col:75> Text="  false, serialize another version (e.g. the result of encodePixels)."
| |-CXXMethodDecl 0x30505d0 <line:35:5, line:37:5> line:35:13 encodePixels 'class SkData *(const struct SkImageInfo &, const void *, size_t)'
| | |-ParmVarDecl 0x30503c0 <col:26, col:45> col:45 used info 'const struct SkImageInfo &'
| | |-ParmVarDecl 0x3050430 <col:51, col:63> col:63 used pixels 'const void *'
| | |-ParmVarDecl 0x30504a0 <col:71, col:78> col:78 used rowBytes 'size_t':'unsigned long'
| | |-CompoundStmt 0x30511e8 <col:88, line:37:5>
| | | '-ReturnStmt 0x30511c8 <line:36:9, col:59>
| | |   '-CXXMemberCallExpr 0x3051158 <col:16, col:59> 'class SkData *'
| | |     |-MemberExpr 0x30510b0 <col:16, col:22> '<bound member method type>' ->onEncodePixels 0x3050a80
| | |     | '-CXXThisExpr 0x3051098 <col:16> 'class SkPixelSerializer *' this
| | |     |-DeclRefExpr 0x30510e0 <col:37> 'const struct SkImageInfo' lvalue ParmVar 0x30503c0 'info' 'const struct SkImageInfo &'
| | |     |-ImplicitCastExpr 0x3051198 <col:43> 'const void *' <LValueToRValue>
| | |     | '-DeclRefExpr 0x3051108 <col:43> 'const void *' lvalue ParmVar 0x3050430 'pixels' 'const void *'
| | |     '-ImplicitCastExpr 0x30511b0 <col:51> 'size_t':'unsigned long' <LValueToRValue>
| | |       '-DeclRefExpr 0x3051130 <col:51> 'size_t':'unsigned long' lvalue ParmVar 0x30504a0 'rowBytes' 'size_t':'unsigned long'
| | '-FullComment 0x38188e0 <line:32:7, line:33:47>
| |   '-ParagraphComment 0x38188b0 <line:32:7, line:33:47>
| |     |-TextComment 0x3818860 <line:32:7, col:72> Text="  Call to get the client's version of encoding these pixels. If it"
| |     '-TextComment 0x3818880 <line:33:7, col:47> Text="  returns NULL, serialize the raw pixels."
| |-AccessSpecDecl 0x30506c0 <line:39:1, col:10> col:1 protected
| |-CXXMethodDecl 0x3050800 <line:44:5, col:67> col:18 referenced onUseEncodedData '_Bool (const void *, size_t)' virtual pure
| | |-ParmVarDecl 0x3050700 <col:35, col:47> col:47 data 'const void *'
| | |-ParmVarDecl 0x3050770 <col:53, col:60> col:60 len 'size_t':'unsigned long'
| | '-FullComment 0x38189d0 <line:41:7, line:42:69>
| |   '-ParagraphComment 0x38189a0 <line:41:7, line:42:69>
| |     |-TextComment 0x3818950 <line:41:7, col:80> Text="  Return true if you want to serialize the encoded data, false if you want"
| |     '-TextComment 0x3818970 <line:42:7, col:69> Text="  another version serialized (e.g. the result of encodePixels)."
| |-CXXMethodDecl 0x3050a80 <line:50:5, col:95> col:21 referenced onEncodePixels 'class SkData *(void) const'
| | |-ParmVarDecl 0x3050910 <col:36, col:53> col:54 'const struct SkImageInfo &'
| | |-ParmVarDecl 0x3050980 <col:56, col:68> col:68 pixels 'const void *'
| | |-ParmVarDecl 0x30509f0 <col:76, col:83> col:83 rowBytes 'size_t':'unsigned long'
| | '-FullComment 0x3818ac0 <line:47:7, line:48:60>
| |   '-ParagraphComment 0x3818a90 <line:47:7, line:48:60>
| |     |-TextComment 0x3818a40 <line:47:7, col:80> Text="  If you want to encode these pixels, return the encoded data as an SkData"
| |     '-TextComment 0x3818a60 <line:48:7, col:60> Text="  Return null if you want to serialize the raw pixels."
| |-CXXConstructorDecl 0x3050bf0 <line:19:7> col:7 implicit SkPixelSerializer 'void (const class SkPixelSerializer &)' inline noexcept-unevaluated 0x3050bf0
| | '-ParmVarDecl 0x3050d30 <col:7> col:7 'const class SkPixelSerializer &'
| '-CXXMethodDecl 0x3050dc0 <col:7, <invalid sloc>> col:7 implicit operator= 'class SkPixelSerializer &(const class SkPixelSerializer &)' inline noexcept-unevaluated 0x3050dc0
|   '-ParmVarDecl 0x3050ee0 <col:7> col:7 'const class SkPixelSerializer &'
| |-CXXMethodDecl 0x3050a80 <line:50:5, col:95> col:21 referenced onEncodePixels 'class SkData *(void) const'
| | |-ParmVarDecl 0x3050910 <col:36, col:53> col:54 'const struct SkImageInfo &'
| | |-ParmVarDecl 0x3050980 <col:56, col:68> col:68 pixels 'const void *'
| | |-ParmVarDecl 0x30509f0 <col:76, col:83> col:83 rowBytes 'size_t':'unsigned long'
| | '-FullComment 0x3818ac0 <line:47:7, line:48:60>
| |   '-ParagraphComment 0x3818a90 <line:47:7, line:48:60>
| |     |-TextComment 0x3818a40 <line:47:7, col:80> Text="  If you want to encode these pixels, return the encoded data as an SkData"
| |     '-TextComment 0x3818a60 <line:48:7, col:60> Text="  Return null if you want to serialize the raw pixels."
| |-CXXConstructorDecl 0x3050bf0 <line:19:7> col:7 implicit SkPixelSerializer 'void (const class SkPixelSerializer &)' inline noexcept-unevaluated 0x3050bf0
| | '-ParmVarDecl 0x3050d30 <col:7> col:7 'const class SkPixelSerializer &'
| '-CXXMethodDecl 0x3050dc0 <col:7, <invalid sloc>> col:7 implicit operator= 'class SkPixelSerializer &(const class SkPixelSerializer &)' inline noexcept-unevaluated 0x3050dc0
|   '-ParmVarDecl 0x3050ee0 <col:7> col:7 'const class SkPixelSerializer &'
|-CXXRecordDecl 0x304fcd0 prev 0x2f34d70 <../../include/core/SkPatch3D.cpp:13:1, col:7> col:7 referenced class SkData
| |-CXXRecordDecl 0x226fb90 <col:1, col:7> col:7 implicit referenced class SkPatch3D
| |-AccessSpecDecl 0x226fc20 <line:74:1, col:7> col:1 public
| |-CXXConstructorDecl 0x226fcc0 <line:75:5, col:15> col:5 SkPatch3D 'void (void)'
| |-CXXMethodDecl 0x226fda0 <line:77:5, col:19> col:13 reset 'void (void)'
| |-CXXMethodDecl 0x226ffc0 <line:78:5, col:65> col:13 transform 'void (const struct SkMatrix3D &, class SkPatch3D *) const'
| | |-ParmVarDecl 0x226fe50 <col:23, col:39> col:40 'const struct SkMatrix3D &'
| | '-ParmVarDecl 0x226fec0 <col:42, /usr/lib/llvm-3.6/bin/../lib/clang/3.6.0/include/stddef.h:100:18> ../../include/utils/SkCamera.h:78:53 dst 'class SkPatch3D *' cinit
| |   '-ImplicitCastExpr 0x2270c80 </usr/lib/llvm-3.6/bin/../lib/clang/3.6.0/include/stddef.h:100:18> 'class SkPatch3D *' <NullToPointer>
| |     '-GNUNullExpr 0x2270c68 <col:18> 'long'
| |-CXXMethodDecl 0x2270290 <../../include/utils/SkCamera.h:81:5, col:61> col:14 used dotWith 'SkScalar (SkScalar, SkScalar, enum SkPath::Direction) const'
| | |-ParmVarDecl 0x2270080 <col:22, col:31> col:31 dx 'SkScalar':'float'
| | |-ParmVarDecl 0x22700f0 <col:35, col:44> col:44 dy 'SkScalar':'float'
| | '-ParmVarDecl 0x2270160 <col:48, col:57> col:57 dz 'SkScalar':'float'
| |-CXXMethodDecl 0x2270460 <line:82:5, line:84:5> line:82:14 DotWith 'SkScalar (const SkVector3D &) const' static
| | |-ParmVarDecl 0x2270360 <col:22, col:40> col:40 used v 'const SkVector3D &'
| | '-CompoundStmt 0x2270f00 <col:49, line:84:5>
| |-CXXMethodDecl 0x368eed0 <../../include/core/SkRect.h:47:5, line:51:5> line:47:42 used MakeLTRB 'struct SkPatch3D (int32_t, int32_t, int32_t, int32_t)' static
| |-CXXMethodDecl 0x3b29660 <line:94:5, line:99:5> line:94:14 operator- 'struct SkIPoint (void) const'
| | |-CompoundStmt 0x3b2c300 <col:32, line:99:5>
| | | |-DeclStmt 0x3b2bf98 <line:95:9, col:21>
| | | | '-VarDecl 0x3b2bf10 <col:9, col:18> col:18 used neg 'struct SkIPoint' nrvo callinit
| | |-CXXConstructorDecl 0x397e6f0 <line:591:5, line:594:5> line:591:5 SkAutoSMalloc<kSize> 'void (void)'`
