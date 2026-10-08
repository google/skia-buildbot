package ingester

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	db_mocks "go.skia.org/infra/autogardener/go/db/mocks"
	gemini_mocks "go.skia.org/infra/autogardener/go/gemini/mocks"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/autogardener/go/utils"
	"go.skia.org/infra/go/util"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
)

func TestClassifyExtractedError(t *testing.T) {
	ctx := t.Context()

	setup := func(t *testing.T) (*db_mocks.AutoGardenerDB, *gemini_mocks.Client, *errorClassifier, *ts_types.Task, *types.ExtractedError) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		c := newErrorClassifier(mockDB, mockG)

		task := &ts_types.Task{
			Id: "task1",
			TaskKey: ts_types.TaskKey{
				RepoState: ts_types.RepoState{
					Repo: "my_repo",
				},
			},
		}
		extractedErr := &types.ExtractedError{
			Analysis:     "analysis",
			ErrorMessage: "Compilation failed due to missing semicolon in SkCanvas.cpp:123",
		}
		return mockDB, mockG, c, task, extractedErr
	}

	// Case 1: No previous matching failure classes exist.
	// This results in a new failure class being registered, without calling into Gemini.
	t.Run("no existing failure class", func(t *testing.T) {
		mockDB, mockG, c, task, extractedErr := setup(t)

		mockDB.On("GetExtractedError", ctx, task.Id).Return(extractedErr, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return([]*types.FailureClass{}, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, mock.MatchedBy(func(fc *types.FailureClass) bool {
			return fc.Repo == task.Repo && fc.ErrorMessage() == extractedErr.ErrorMessage && fc.Analysis() == extractedErr.Analysis && fc.Id != ""
		})).Return(nil).Once()
		mockDB.On("PutExtractedError", mock.Anything, task.Id, mock.MatchedBy(func(ee *types.ExtractedError) bool {
			return ee.FailureClassId != ""
		})).Return(nil).Once()

		require.NoError(t, c.classifyExtractedError(ctx, task, extractedErr))
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 2: Failure class with exactly-matching error message. No need to
	// call into Gemini.
	t.Run("has exactly-matching failure class", func(t *testing.T) {
		mockDB, mockG, c, task, extractedErr := setup(t)

		fc := &types.FailureClass{
			Id:   "class_abc",
			Repo: task.Repo,
			Updates: []*types.FailureClassUpdate{
				{
					ErrorMessage: types.Ptr(extractedErr.ErrorMessage),
				},
			},
		}
		failureClasses := []*types.FailureClass{fc}
		mockDB.On("GetExtractedError", ctx, task.Id).Return(extractedErr, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return(failureClasses, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, fc).Return(nil).Once()
		mockDB.On("PutExtractedError", mock.Anything, task.Id, mock.MatchedBy(func(ee *types.ExtractedError) bool {
			return ee.FailureClassId == "class_abc"
		})).Return(nil).Once()

		require.NoError(t, c.classifyExtractedError(ctx, task, extractedErr))
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 3: Matching failure class exists, Gemini returns its ID.
	t.Run("gemini assigned failure class", func(t *testing.T) {
		mockDB, mockG, c, task, extractedErr := setup(t)

		fc := &types.FailureClass{
			Id:   "class_abc",
			Repo: task.Repo,
			Updates: []*types.FailureClassUpdate{
				{
					ErrorMessage: types.Ptr("task failed with: " + extractedErr.ErrorMessage),
				},
			},
		}
		failureClasses := []*types.FailureClass{fc}
		mockDB.On("GetExtractedError", ctx, task.Id).Return(extractedErr, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return(failureClasses, nil).Once()
		mockG.On("ClassifyFailure", mock.Anything, extractedErr, failureClasses, task.Repo).Return("class_abc", nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, fc).Return(nil).Once()
		mockDB.On("PutExtractedError", mock.Anything, task.Id, mock.MatchedBy(func(ee *types.ExtractedError) bool {
			return ee.FailureClassId == "class_abc"
		})).Return(nil).Once()

		require.NoError(t, c.classifyExtractedError(ctx, task, extractedErr))
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 4: One error contains the other, with a massive amount of extra
	// context. Jaccard similarity is low, but overlap is high, so we still
	// consider the candidate.
	t.Run("overlap candidate assigned by gemini", func(t *testing.T) {
		mockDB, mockG, c, task, _ := setup(t)

		shortErr := "../../../../../skia/tests/MultiPictureDocumentTest.cpp:426	 [SkMultiPictureDocument_AHardwarebuffer, Vulkan]: ToolUtils::equal_pixels(img.get(), expectedImages[0].get())"
		longErr := `
[560/635] unit test  SkSLMetalTest_Ganesh done
[561/635] unit test  GrSurfaceProxyTest done
[562/635] unit test  PathOpsOpTest_Threaded done
[563/635] unit test  ParagraphTest_FontCollection done
[564/635] unit test  SkRuntimeEffectTest_Blender done
[565/635] unit test  UnicodeTest_GraphemeCluster done
[566/635] unit test  MeshTest_VertexSpecification done
[567/635] unit test  CodecTest_AnimatedWebP done
[568/635] unit test  ShadowUtilsTest_AmbientSpot done
[569/635] unit test  ColorSpaceTest_TransferFn done
[570/635] unit test  WritePixelsTest_AlphaType done
[571/635] unit test  ReadPixelsTest_Dimensions done
[572/635] unit test  SurfaceTest_SnapRRect done
[573/635] unit test  ImageFilterTest_MatrixConvolution done
[574/635] unit test  FontMgrTest_MatchFamilyStyle done
[575/635] unit test  ClipStackTest_IsRRect done
[576/635] unit test  PictureShaderTest_LocalMatrix done
[577/635] unit test  VerticesTest_AttributeCount done
[578/635] unit test  AnnotationTest_PdfLink done
[579/635] unit test  BigMatrixTest_Perspective done
[580/635] unit test  CanvasTest_SaveLayerRec done
[581/635] unit test  RegionTest_OpDifference done
[582/635] unit test  ShaderTest_LinearGradient done
[583/635] unit test  MatrixTest_ConcatRotateScale done
[584/635] unit test  PaintTest_StrokeMiterLimit done
[585/635] unit test  PathTest_AddRoundRect done
[586/635] unit test  RecordDrawTest_CullRect done
[587/635] unit test  SVGDeviceTest_ClipPath done
[588/635] unit test  TextBlobTest_Intercepts done
[589/635] unit test  TypefaceTest_GetKerningPairAdjustments done
[590/635] unit test  MatrixProcsTest_Perspective done
[591/635] unit test  GeometryTest_QuadChopAt done
[592/635] unit test  DataRefTest_FromFile done
[593/635] unit test  StreamTest_MemoryStream done
[594/635] unit test  FlattenableTest_FactorySerialization done
[595/635] unit test  FilterResult_ganesh_MakeFromImage done
[596/635] unit test  FilterResult_ganesh_RescaleWithColorFilter done
[597/635] unit test  FilterResult_ganesh_RescaleWithTransform done
[598/635] unit test  FilterResult_ganesh_RescaleWithTileMode done
[599/635] unit test  FilterResult_ganesh_BackdropFilterRotated done
[600/635] unit test  FilterResult_ganesh_CroppedTransformedTransparencyAffectingColorFilter done
[601/635] unit test  FilterResult_ganesh_CroppedTransformedColorFilter done
[602/635] unit test  FilterResult_ganesh_ColorFilterBetweenCrops done
[603/635] unit test  FilterResult_ganesh_CropBetweenColorFilters done
[604/635] unit test  FilterResult_ganesh_CroppedColorFilter done
[605/635] unit test  FilterResult_ganesh_ColorFilterBetweenTransforms done
[606/635] unit test  FilterResult_ganesh_TransformBetweenColorFilters done
[607/635] unit test  FilterResult_ganesh_TransformedColorFilter done
[608/635] unit test  FilterResult_ganesh_ColorFilter done
[609/635] unit test  FilterResult_ganesh_TransformAndTile done
[610/635] unit test  FilterResult_ganesh_TransformAndCrop done
[611/635] unit test  FilterResult_ganesh_TransformBecomesEmpty done
[612/635] unit test  FilterResult_ganesh_IntegerOffsetIgnoresNearestSampling done
[613/635] unit test  FilterResult_ganesh_IncompatibleSamplingResolvesImages done
[614/635] unit test  FilterResult_ganesh_CompatibleSamplingConcatsTransforms done
[615/635] unit test  FilterResult_ganesh_Transform done
[616/635] unit test  FilterResult_ganesh_DecalThenClamp done
[617/635] unit test  FilterResult_ganesh_PeriodicTileCrops done
[618/635] unit test  FilterResult_ganesh_IntersectingCrops done
[619/635] unit test  FilterResult_ganesh_DisjointCrops done
[620/635] unit test  FilterResult_ganesh_EmptyCrop done
[621/635] unit test  FilterResult_ganesh_CropDisjointFromSourceAndOutput done
[622/635] unit test  FilterResult_ganesh_Crop done
[623/635] unit test  FilterResult_ganesh_EmptyDesiredOutput done
[624/635] unit test  FilterResult_ganesh_EmptySource done
[625/635] unit test  F16DrawTest_Ganesh done
[626/635] unit test  ExtendedSkColorTypeTests_gpu done
	start unit test  DirectMaskLimitTest_Ganesh
[627/635] unit test  DirectMaskLimitTest_Ganesh done
	start unit test  SpecialImage_GPUDevice
[628/635] unit test  SpecialImage_GPUDevice done
	start unit test  ComposeFailureWithInputElision
[629/635] unit test  ComposeFailureWithInputElision done
	start unit test  TestManyDrawsGanesh
[630/635] unit test  TestManyDrawsGanesh done
	start unit test  BlurDegenerateAffineFuzzer
[631/635] unit test  BlurDegenerateAffineFuzzer done
[632/635] unit test  BlurMaskBiggerThanDest done
	start unit test  SmallBoxBlurBug
[633/635] unit test  SmallBoxBlurBug done
[634/635] unit test  TiledDrawCacheTest_Ganesh done
[635/635] unit test  BigImageTest_Ganesh done

[635/635] 150MB RAM, 460MB peak, 0 queued, 1 threads:
	unit test  BigImageTest_Ganesh  done

[635/635] 150MB RAM, 460MB peak, 0 queued, 1 threads:
	unit test  BigImageTest_Ganesh  done
Failures:
	../../../../../skia/tests/MultiPictureDocumentTest.cpp:426	 [SkMultiPictureDocument_AHardwarebuffer, Vulkan]: ToolUtils::equal_pixels(img.get(), expectedImages[0].get())
1 failures
+ >/data/local/tmp/rc
+ echo 1
`
		shortErrSanitized := utils.SanitizeErrorText(shortErr)
		longErrSanitized := utils.SanitizeErrorText(longErr)
		require.Less(t, util.NgramSimilarity(shortErrSanitized, longErrSanitized, ngramSize), ngramSimilarityCandidateThreshold)
		require.Greater(t, util.NgramOverlap(shortErrSanitized, longErrSanitized, ngramSize), ngramOverlapCandidateThreshold)

		extractedErr := &types.ExtractedError{
			Analysis:     "analysis",
			ErrorMessage: longErr,
		}

		fc := &types.FailureClass{
			Id:   "overlap-failure-class",
			Repo: task.Repo,
			Updates: []*types.FailureClassUpdate{
				{
					ErrorMessage: types.Ptr(shortErr),
				},
			},
		}
		failureClasses := []*types.FailureClass{fc}
		mockDB.On("GetExtractedError", ctx, task.Id).Return(extractedErr, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return(failureClasses, nil).Once()
		mockG.On("ClassifyFailure", mock.Anything, extractedErr, failureClasses, task.Repo).Return(fc.Id, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, fc).Return(nil).Once()
		mockDB.On("PutExtractedError", mock.Anything, task.Id, mock.MatchedBy(func(ee *types.ExtractedError) bool {
			return ee.FailureClassId == fc.Id
		})).Return(nil).Once()

		require.NoError(t, c.classifyExtractedError(ctx, task, extractedErr))
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 5: Shortcut if we already classified the ExtractedError.
	t.Run("already classified", func(t *testing.T) {
		mockDB, mockG, c, task, extractedErr := setup(t)

		alreadyClassified := &types.ExtractedError{
			ErrorMessage:   extractedErr.ErrorMessage,
			Analysis:       extractedErr.Analysis,
			FailureClassId: "some-failure-class",
		}
		mockDB.On("GetExtractedError", ctx, task.Id).Return(alreadyClassified, nil).Once()
		require.NoError(t, c.classifyExtractedError(ctx, task, extractedErr))
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 6: Generic error message shortcuts classification to generic-failure.
	t.Run("generic error message", func(t *testing.T) {
		mockDB, mockG, c, task, _ := setup(t)

		genericExtractedErr := &types.ExtractedError{
			Analysis:     "analysis",
			ErrorMessage: "exit status 1",
		}

		mockDB.On("GetExtractedError", ctx, task.Id).Return(genericExtractedErr, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, mock.MatchedBy(func(fc *types.FailureClass) bool {
			return fc.Id == genericFailureClassID
		})).Return(nil).Once()
		mockDB.On("PutExtractedError", mock.Anything, task.Id, mock.MatchedBy(func(ee *types.ExtractedError) bool {
			return ee.FailureClassId == genericFailureClassID
		})).Return(nil).Once()

		require.NoError(t, c.classifyExtractedError(ctx, task, genericExtractedErr))
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})
}
