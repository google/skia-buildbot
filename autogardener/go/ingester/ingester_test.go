package ingester

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	db_mocks "go.skia.org/infra/autogardener/go/db/mocks"
	gemini_mocks "go.skia.org/infra/autogardener/go/gemini/mocks"
	"go.skia.org/infra/autogardener/go/types"
	"go.skia.org/infra/autogardener/go/utils"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/util"
	td_db "go.skia.org/infra/task_driver/go/db"
	td_mocks "go.skia.org/infra/task_driver/go/db/mocks"
	"go.skia.org/infra/task_driver/go/td"
	ts_db "go.skia.org/infra/task_scheduler/go/db"
	ts_mocks "go.skia.org/infra/task_scheduler/go/mocks"
	ts_types "go.skia.org/infra/task_scheduler/go/types"
)

func TestTaskProcessingRegistry(t *testing.T) {
	r := newTaskProcessingRegistry()

	// 1. First claim succeeds
	release1, ok := r.TryClaimTask("task1")
	require.True(t, ok)
	require.NotNil(t, release1)

	// 2. Second concurrent claim for same task ID fails
	release2, ok := r.TryClaimTask("task1")
	require.False(t, ok)
	require.Nil(t, release2)

	// 3. Releasing first claim allows claiming again
	release1()
	release3, ok := r.TryClaimTask("task1")
	require.True(t, ok)
	require.NotNil(t, release3)
	release3()
}

func TestIngestTask(t *testing.T) {
	ctx := t.Context()
	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	mockTDDB := td_mocks.NewDB(t)
	i := &Ingester{
		db:     mockDB,
		gemini: mockG,
		tdDB:   mockTDDB,
	}

	task := &ts_types.Task{
		Id:       "task1",
		Finished: time.Now().Add(-5 * time.Minute),
	}

	// 1. Task already has a summary in the DB.
	t.Run("already ingested", func(t *testing.T) {
		registry := newTaskProcessingRegistry()
		existing := &types.TaskSummary{
			ErrorMessage: "error",
			Analysis:     "analysis",
		}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(existing, nil).Once()
		taskSummary, err := i.ingestTask(ctx, registry, task, nil)
		require.NoError(t, err)
		require.Equal(t, existing, taskSummary)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})

	// 2. Task needs to be summarized.
	t.Run("not yet ingested", func(t *testing.T) {
		registry := newTaskProcessingRegistry()
		summary := &types.TaskSummary{
			Analysis:     "analysis",
			ErrorMessage: "error",
		}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(nil, nil).Once()
		mockTDDB.On("GetTaskDriver", ctx, task.Id).Return(nil, nil).Once()
		mockG.On("GetTaskSummary", ctx, task).Return(summary, nil).Once()
		mockDB.On("PutTaskSummary", ctx, task.Id, summary).Return(nil).Once()

		taskSummary, err := i.ingestTask(ctx, registry, task, nil)
		require.NoError(t, err)
		require.Equal(t, summary, taskSummary)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})

	// 3. Task is already being processed, return nil, nil.
	t.Run("already being processed", func(t *testing.T) {
		registry := newTaskProcessingRegistry()
		release, ok := registry.TryClaimTask(task.Id)
		require.True(t, ok)
		defer release()

		taskSummary, err := i.ingestTask(ctx, registry, task, nil)
		require.NoError(t, err)
		require.Nil(t, taskSummary)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})
}

func TestClassifyTaskSummary(t *testing.T) {
	ctx := t.Context()

	setup := func(t *testing.T) (*db_mocks.AutoGardenerDB, *gemini_mocks.Client, *Ingester, *ts_types.Task, *types.TaskSummary) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		i := &Ingester{
			db:     mockDB,
			gemini: mockG,
			tdDB:   td_mocks.NewDB(t),
		}

		task := &ts_types.Task{
			Id: "task1",
			TaskKey: ts_types.TaskKey{
				RepoState: ts_types.RepoState{
					Repo: "my_repo",
				},
			},
		}
		taskSummary := &types.TaskSummary{
			Analysis:     "analysis",
			ErrorMessage: "Compilation failed due to missing semicolon in SkCanvas.cpp:123",
		}
		return mockDB, mockG, i, task, taskSummary
	}

	// Case 1: No previous matching failure classes exist.
	// This results in a new failure class being registered, without calling into Gemini.
	t.Run("no existing failure class", func(t *testing.T) {
		mockDB, mockG, i, task, taskSummary := setup(t)

		mockDB.On("GetTaskSummary", ctx, task.Id).Return(taskSummary, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return([]*types.FailureClass{}, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, mock.MatchedBy(func(fc *types.FailureClass) bool {
			return fc.Repo == task.Repo && fc.ErrorMessage == taskSummary.ErrorMessage && fc.Analysis == taskSummary.Analysis && fc.Id != ""
		})).Return(nil).Once()
		mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.MatchedBy(func(ts *types.TaskSummary) bool {
			return ts.FailureClassId != ""
		})).Return(nil).Once()

		err := i.classifyTaskSummary(ctx, task, taskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 2: Failure class with exactly-matching error message. No need to
	// call into Gemini.
	t.Run("has exactly-matching failure class", func(t *testing.T) {
		mockDB, mockG, i, task, taskSummary := setup(t)

		fc := &types.FailureClass{
			Id:           "class_abc",
			Repo:         task.Repo,
			ErrorMessage: taskSummary.ErrorMessage,
		}
		failureClasses := []*types.FailureClass{fc}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(taskSummary, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return(failureClasses, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, fc).Return(nil).Once()
		mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.MatchedBy(func(ts *types.TaskSummary) bool {
			return ts.FailureClassId == "class_abc"
		})).Return(nil).Once()

		err := i.classifyTaskSummary(ctx, task, taskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 3: Matching failure class exists, Gemini returns its ID.
	t.Run("gemini assigned failure class", func(t *testing.T) {
		mockDB, mockG, i, task, taskSummary := setup(t)

		fc := &types.FailureClass{
			Id:           "class_abc",
			Repo:         task.Repo,
			ErrorMessage: "task failed with: " + taskSummary.ErrorMessage,
		}
		failureClasses := []*types.FailureClass{fc}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(taskSummary, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return(failureClasses, nil).Once()
		mockG.On("ClassifyFailure", mock.Anything, taskSummary, failureClasses, task.Repo).Return("class_abc", nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, fc).Return(nil).Once()
		mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.MatchedBy(func(ts *types.TaskSummary) bool {
			return ts.FailureClassId == "class_abc"
		})).Return(nil).Once()

		err := i.classifyTaskSummary(ctx, task, taskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 4: One error contains the other, with a massive amount of extra
	// context. Jaccard similarity is low, but overlap is high, so we still
	// consider the candidate.
	t.Run("overlap candidate assigned by gemini", func(t *testing.T) {
		mockDB, mockG, i, task, _ := setup(t)

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
		// Self-test: ensure that the ngram similarity and overlap are under and
		// over their respective thresholds.
		shortErrSanitized := utils.SanitizeErrorText(shortErr)
		longErrSanitized := utils.SanitizeErrorText(longErr)
		require.Less(t, util.NgramSimilarity(shortErrSanitized, longErrSanitized, ngramSize), ngramSimilarityCandidateThreshold)
		require.Greater(t, util.NgramOverlap(shortErrSanitized, longErrSanitized, ngramSize), ngramOverlapCandidateThreshold)

		taskSummary := &types.TaskSummary{
			Analysis:     "analysis",
			ErrorMessage: longErr,
		}

		fc := &types.FailureClass{
			Id:           "overlap-failure-class",
			Repo:         task.Repo,
			ErrorMessage: shortErr,
		}
		failureClasses := []*types.FailureClass{fc}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(taskSummary, nil).Once()
		mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return(failureClasses, nil).Once()
		mockG.On("ClassifyFailure", mock.Anything, taskSummary, failureClasses, task.Repo).Return(fc.Id, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, fc).Return(nil).Once()
		mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.MatchedBy(func(ts *types.TaskSummary) bool {
			return ts.FailureClassId == fc.Id
		})).Return(nil).Once()

		err := i.classifyTaskSummary(ctx, task, taskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 5: Shortcut if we already classified the TaskSummary.
	t.Run("already classified", func(t *testing.T) {
		mockDB, mockG, i, task, taskSummary := setup(t)

		alreadyClassified := &types.TaskSummary{
			ErrorMessage:   taskSummary.ErrorMessage,
			Analysis:       taskSummary.Analysis,
			FailureClassId: "some-failure-class",
		}
		mockDB.On("GetTaskSummary", ctx, task.Id).Return(alreadyClassified, nil).Once()
		err := i.classifyTaskSummary(ctx, task, taskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})

	// Case 6: Generic error message shortcuts classification to generic-unidentified-failure.
	t.Run("generic error message", func(t *testing.T) {
		mockDB, mockG, i, task, _ := setup(t)

		genericTaskSummary := &types.TaskSummary{
			Analysis:     "analysis",
			ErrorMessage: "exit status 1",
		}

		mockDB.On("GetTaskSummary", ctx, task.Id).Return(genericTaskSummary, nil).Once()
		mockDB.On("PutFailureClass", mock.Anything, mock.MatchedBy(func(fc *types.FailureClass) bool {
			return fc.Id == genericFailureClassID
		})).Return(nil).Once()
		mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.MatchedBy(func(ts *types.TaskSummary) bool {
			return ts.FailureClassId == genericFailureClassID
		})).Return(nil).Once()

		err := i.classifyTaskSummary(ctx, task, genericTaskSummary)
		require.NoError(t, err)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
	})
}

func TestEnqueueModifiedTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockTSDB := ts_mocks.NewRemoteDB(t)
	repoURL := "http://my-repo.git"
	i := &Ingester{
		tsDB: mockTSDB,
	}

	modCh := make(chan []*ts_types.Task)
	mockTSDB.On("ModifiedTasksCh", mock.Anything).Return((<-chan []*ts_types.Task)(modCh))

	taskCh := make(chan *ts_types.Task)

	taskValid := &ts_types.Task{
		Id:       "valid-task",
		Status:   ts_types.TASK_STATUS_FAILURE,
		Finished: time.Now().Add(-10 * time.Minute),
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: repoURL,
			},
		},
	}

	taskMismatchedRepo := &ts_types.Task{
		Id:       "mismatched-repo-task",
		Status:   ts_types.TASK_STATUS_FAILURE,
		Finished: time.Now().Add(-10 * time.Minute),
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: "http://other-repo.git",
			},
		},
	}

	taskRunning := &ts_types.Task{
		Id:       "running-task",
		Status:   ts_types.TASK_STATUS_RUNNING,
		Finished: time.Time{},
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: repoURL,
			},
		},
	}

	taskSuccess := &ts_types.Task{
		Id:       "success-task",
		Status:   ts_types.TASK_STATUS_SUCCESS,
		Finished: time.Now().Add(-10 * time.Minute),
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: repoURL,
			},
		},
	}

	go i.enqueueModifiedTasks(ctx, repoURL, taskCh)

	// Send tasks to modCh. enqueueModifiedTasks processes tasks in the order
	// that they arrive, so sending taskValid last allows us to use its arrival
	// from taskCh to signal that all other tasks have been processed as well.
	modCh <- []*ts_types.Task{taskMismatchedRepo, taskRunning, taskSuccess}
	modCh <- []*ts_types.Task{taskValid}

	// Verify only the valid task is written to taskCh
	popped := <-taskCh
	require.Equal(t, taskValid.Id, popped.Id)
}

func TestPeriodicTaskPollingFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	mockTSDB := ts_mocks.NewRemoteDB(t)
	repoURL := "http://my-repo.git"
	period := 24 * time.Hour

	i := &Ingester{
		db:     mockDB,
		gemini: mockG,
		tsDB:   mockTSDB,
		tdDB:   td_mocks.NewDB(t),
	}

	task := &ts_types.Task{
		Id:     "failed-task",
		Status: ts_types.TASK_STATUS_FAILURE,
	}

	// Query fallback tasks: SearchTasks is called twice (for FAILURE and MISHAP status)
	mockTSDB.On("SearchTasks", mock.Anything, mock.MatchedBy(func(p *ts_db.TaskSearchParams) bool {
		return *p.Status == ts_types.TASK_STATUS_FAILURE
	})).Return([]*ts_types.Task{task}, nil).Once()
	mockTSDB.On("SearchTasks", mock.Anything, mock.MatchedBy(func(p *ts_db.TaskSearchParams) bool {
		return *p.Status == ts_types.TASK_STATUS_MISHAP
	})).Return([]*ts_types.Task{}, nil).Once()

	taskCh := make(chan *ts_types.Task)

	go i.periodicTaskPollingFallback(ctx, repoURL, period, taskCh)

	popped := <-taskCh
	require.Equal(t, task.Id, popped.Id)
}

func TestIngestTasks(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	mockTDDB := td_mocks.NewDB(t)
	i := &Ingester{
		db:     mockDB,
		gemini: mockG,
		tdDB:   mockTDDB,
	}

	task1 := &ts_types.Task{
		Id: "task1",
	}
	summary := &types.TaskSummary{
		Analysis: "analysis",
	}

	mockDB.On("GetTaskSummary", mock.Anything, task1.Id).Return(nil, nil).Once()
	mockTDDB.On("GetTaskDriver", mock.Anything, task1.Id).Return(nil, nil).Once()
	mockG.On("GetTaskSummary", mock.Anything, task1).Return(summary, nil).Once()
	mockDB.On("PutTaskSummary", mock.Anything, task1.Id, summary).Return(nil).Once()

	task2 := &ts_types.Task{
		Id: "task2",
	}
	someError := errors.New("uh oh")
	mockDB.On("GetTaskSummary", mock.Anything, task2.Id).Return(nil, nil).Once()
	mockTDDB.On("GetTaskDriver", mock.Anything, task2.Id).Return(nil, nil).Once()
	mockG.On("GetTaskSummary", mock.Anything, task2).Return(nil, someError).Once()

	inputCh := make(chan *ts_types.Task)
	outputCh := make(chan *taskIngestionResult)

	go i.ingestTasks(ctx, inputCh, outputCh)

	inputCh <- task1
	result1 := <-outputCh
	require.NoError(t, result1.err)
	require.Equal(t, task1.Id, result1.Task.Id)
	require.Equal(t, summary.Analysis, result1.Summary.Analysis)

	inputCh <- task2
	result2 := <-outputCh
	require.ErrorContains(t, result2.err, someError.Error())
	require.Nil(t, result2.Task)
	require.Nil(t, result2.Summary)
}

func TestPeriodicUnclassifiedTaskSummaryPollingFallback(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockTSDB := ts_mocks.NewRemoteDB(t)
	i := &Ingester{
		db:   mockDB,
		tsDB: mockTSDB,
	}

	task := &ts_types.Task{
		Id: "unclassified-task",
	}
	summary := &types.TaskSummary{
		Analysis: "unclassified analysis",
	}

	mockDB.On("GetUnclassifiedTaskSummaries", mock.Anything, getUnclassifiedTaskSummariesBatchSize).Return(map[string]*types.TaskSummary{
		task.Id: summary,
	}, nil).Once()
	mockTSDB.On("GetTaskById", mock.Anything, task.Id).Return(task, nil).Once()

	classifyCh := make(chan *types.TaskAndSummary)

	go i.periodicUnclassifiedTaskSummaryPollingFallback(ctx, classifyCh)

	item := <-classifyCh
	require.Equal(t, task.Id, item.Task.Id)
	require.Equal(t, summary.Analysis, item.Summary.Analysis)
}

func TestStartIngestingTaskSummariesForRepo(t *testing.T) {
	repoURL := "http://my-repo.git"

	task := &ts_types.Task{
		Id:       "failed-task-123",
		Status:   ts_types.TASK_STATUS_FAILURE,
		Finished: time.Now().Add(-10 * time.Minute),
		TaskKey: ts_types.TaskKey{
			RepoState: ts_types.RepoState{
				Repo: repoURL,
			},
		},
	}

	summary := &types.TaskSummary{
		Analysis:     "test analysis",
		ErrorMessage: "test error",
	}
	failureClass := &types.FailureClass{
		Id: "failure-class-1",
		// Use an error message which doesn't exactly match, to ensure that we
		// call into Gemini.
		ErrorMessage: "task failed with: " + summary.ErrorMessage,
	}

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	mockDB := db_mocks.NewAutoGardenerDB(t)
	mockG := gemini_mocks.NewClient(t)
	mockTSDB := ts_mocks.NewRemoteDB(t)

	modCh := make(chan []*ts_types.Task)
	mockTSDB.On("ModifiedTasksCh", mock.Anything).Return((<-chan []*ts_types.Task)(modCh))
	mockTSDB.On("SearchTasks", mock.Anything, mock.Anything).Return([]*ts_types.Task{}, nil).Maybe()

	// We must use .Maybe() because the backup unclassified repeat loop runs asynchronously in a concurrent
	// background goroutine immediately upon StartIngestingTaskSummariesForRepo starting.
	// It may or may not execute before the test's context is canceled and asserts expectations,
	// making its execution non-deterministic during the test run.
	mockDB.On("GetUnclassifiedTaskSummaries", mock.Anything, getUnclassifiedTaskSummariesBatchSize).Return(map[string]*types.TaskSummary{}, nil).Maybe()

	mockTDDB := td_mocks.NewDB(t)
	mockTDDB.On("GetTaskDriver", mock.Anything, task.Id).Return(nil, nil).Once()

	i := &Ingester{
		db:     mockDB,
		gemini: mockG,
		tsDB:   mockTSDB,
		tdDB:   mockTDDB,
	}

	// Step 1: Ingest task summary
	mockDB.On("GetTaskSummary", mock.Anything, task.Id).Return(nil, nil).Once()
	mockG.On("GetTaskSummary", mock.Anything, task).Return(summary, nil).Once()
	mockDB.On("PutTaskSummary", mock.Anything, task.Id, summary).Return(nil).Once()

	// Step 2: Classify task summary
	mockDB.On("GetTaskSummary", mock.Anything, task.Id).Return(summary, nil).Once()
	mockDB.On("GetRecentFailureClasses", mock.Anything, task.Repo, mock.Anything, 0).Return([]*types.FailureClass{failureClass}, nil).Once()
	mockG.On("ClassifyFailure", mock.Anything, summary, []*types.FailureClass{failureClass}, task.Repo).Return(failureClass.Id, nil).Once()
	mockDB.On("PutFailureClass", mock.Anything, mock.Anything).Return(nil).Once()

	// The final call to PutTaskSummary indicates that the ingestion is
	// finished. We'll close doneCh to reflect that.
	doneCh := make(chan struct{})
	mockDB.On("PutTaskSummary", mock.Anything, task.Id, mock.Anything).Return(nil).Run(func(args mock.Arguments) {
		close(doneCh)
	}).Once()

	i.StartIngestingTaskSummariesForRepo(ctx, repoURL, 24*time.Hour)

	// Send the task to modCh to trigger ingestion
	modCh <- []*ts_types.Task{task}

	// Wait for the whole pipeline to finish
	<-doneCh // Wait for ingestion to complete.
	mockDB.AssertExpectations(t)
	mockG.AssertExpectations(t)
	mockTSDB.AssertExpectations(t)
}

func TestIngestTask_TaskDriverStepsCheck(t *testing.T) {
	mockTime := time.Date(2026, 9, 9, 12, 0, 0, 0, time.UTC)
	ctx := context.WithValue(t.Context(), now.ContextKey, mockTime)

	// Mock sleepFn to no-op.
	oldSleep := sleepFn
	sleepFn = func(d time.Duration) {}
	defer func() {
		sleepFn = oldSleep
	}()

	t.Run("all steps finished", func(t *testing.T) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		mockTDDB := td_mocks.NewDB(t)

		taskID := "task-all-finished"
		mockTDDB.On("GetTaskDriver", ctx, taskID).Return(&td_db.TaskDriverRun{
			TaskId: taskID,
			Steps: map[string]*td_db.Step{
				"step-1": {
					Properties: &td.StepProperties{
						Id:   "step-1",
						Name: "Step 1",
					},
					Finished: mockTime.Add(-5 * time.Minute),
				},
			},
		}, nil).Once()

		i := &Ingester{
			db:     mockDB,
			gemini: mockG,
			tdDB:   mockTDDB,
		}

		task := &ts_types.Task{
			Id:       taskID,
			Finished: mockTime.Add(-5 * time.Minute),
		}

		summary := &types.TaskSummary{
			Analysis:     "analysis",
			ErrorMessage: "error",
		}

		mockDB.On("GetTaskSummary", ctx, task.Id).Return(nil, nil).Once()
		mockG.On("GetTaskSummary", ctx, task).Return(summary, nil).Once()
		mockDB.On("PutTaskSummary", ctx, task.Id, summary).Return(nil).Once()

		taskSummary, err := i.ingestTask(ctx, newTaskProcessingRegistry(), task, nil)
		require.NoError(t, err)
		require.Equal(t, summary, taskSummary)
		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})

	t.Run("unfinished steps re-enqueue", func(t *testing.T) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		mockTDDB := td_mocks.NewDB(t)

		taskID := "task-unfinished"
		mockTDDB.On("GetTaskDriver", ctx, taskID).Return(&td_db.TaskDriverRun{
			TaskId: taskID,
			Steps: map[string]*td_db.Step{
				"step-1": {
					Properties: &td.StepProperties{
						Id:   "step-1",
						Name: "Step 1",
					},
				},
			},
		}, nil).Once()

		i := &Ingester{
			db:     mockDB,
			gemini: mockG,
			tdDB:   mockTDDB,
		}

		task := &ts_types.Task{
			Id:       taskID,
			Finished: mockTime.Add(-1 * time.Minute), // well within the 5 min timeout
		}

		mockDB.On("GetTaskSummary", ctx, task.Id).Return(nil, nil).Once()

		taskCh := make(chan *ts_types.Task, 1)

		taskSummary, err := i.ingestTask(ctx, newTaskProcessingRegistry(), task, taskCh)
		require.NoError(t, err)
		require.Nil(t, taskSummary)

		// Verify task was re-enqueued after sleep
		select {
		case popped := <-taskCh:
			require.Equal(t, task.Id, popped.Id)
		case <-time.After(100 * time.Millisecond):
			t.Fatal("timed out waiting for task to be re-enqueued")
		}

		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})

	t.Run("unfinished steps exceeded timeout", func(t *testing.T) {
		mockDB := db_mocks.NewAutoGardenerDB(t)
		mockG := gemini_mocks.NewClient(t)
		mockTDDB := td_mocks.NewDB(t)

		taskID := "task-unfinished-exceeded"
		mockTDDB.On("GetTaskDriver", ctx, taskID).Return(&td_db.TaskDriverRun{
			TaskId: taskID,
			Steps: map[string]*td_db.Step{
				"step-1": {
					Properties: &td.StepProperties{
						Id:   "step-1",
						Name: "Step 1",
					},
				},
			},
		}, nil).Once()

		i := &Ingester{
			db:     mockDB,
			gemini: mockG,
			tdDB:   mockTDDB,
		}

		task := &ts_types.Task{
			Id:       taskID,
			Finished: mockTime.Add(-15 * time.Minute), // exceeded the 5 min timeout
		}

		summary := &types.TaskSummary{
			Analysis:     "analysis",
			ErrorMessage: "error",
		}

		mockDB.On("GetTaskSummary", ctx, task.Id).Return(nil, nil).Once()
		mockG.On("GetTaskSummary", ctx, task).Return(summary, nil).Once()
		mockDB.On("PutTaskSummary", ctx, task.Id, summary).Return(nil).Once()

		taskCh := make(chan *ts_types.Task, 1)

		taskSummary, err := i.ingestTask(ctx, newTaskProcessingRegistry(), task, taskCh)
		require.NoError(t, err)
		require.Equal(t, summary, taskSummary)

		// Verify no task was re-enqueued
		select {
		case <-taskCh:
			t.Fatal("task was unexpectedly re-enqueued")
		default:
		}

		mockDB.AssertExpectations(t)
		mockG.AssertExpectations(t)
		mockTDDB.AssertExpectations(t)
	})
}
