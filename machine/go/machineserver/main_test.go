package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/alogin/proxylogin"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/kube/go/authproxy"
	"go.skia.org/infra/machine/go/machine"
	changeSinkMocks "go.skia.org/infra/machine/go/machine/change/sink/mocks"
	"go.skia.org/infra/machine/go/machine/store/mocks"
	"go.skia.org/infra/machine/go/machineserver/rpc"
)

var (
	fakeTime = time.Date(2021, time.September, 1, 2, 3, 4, 0, time.UTC)

	errFake = errors.New("my fake error")

	suppliedDimensions = machine.SwarmingDimensions{
		"mykey": []string{"myvalue"},
	}

	suppliedDimensions2 = machine.SwarmingDimensions{
		"mykey2": []string{"myvalue2"},
	}
)

const (
	machineID = "skia-rpi2-rack4-shelf1-001"

	testUser = "somebody@example.org"

	sshUserIP = "root@skia-spin513-001"

	sshUserIP2 = "chrome-bot@skia-spin513-002"
)

func setupForTest(t *testing.T) (context.Context, machine.Description, *server, *mux.Router, *httptest.ResponseRecorder) {
	ctx := now.TimeTravelingContext(fakeTime)
	desc := machine.NewDescription(ctx)
	desc.Dimensions = machine.SwarmingDimensions{
		machine.DimID: []string{machineID},
	}
	desc.Temperature = map[string]float64{
		"cpu": 27.3,
	}
	desc.SuppliedDimensions = suppliedDimensions.Copy()
	desc.SSHUserIP = sshUserIP

	storeMock := mocks.NewStore(t)
	changeSinkMock := changeSinkMocks.NewSink(t)

	s := &server{
		store: storeMock,

		// Note we use changeSinkMock for both pubsubChangeSink and
		// sserChangeSink, which is why all the changeSinkMock.On("Send",...)
		// have a ``.Times(2)`` on them. This keeps the code simpler for when we
		// eventually drop pubsubChangeSink, which will be right after this CL
		// rolls out to all TMM instance.
		pubsubChangeSink: changeSinkMock,
		sserChangeSink:   changeSinkMock,

		login: proxylogin.NewWithDefaults(),
	}

	// Put a mux.Router in place so the request path gets parsed.
	router := mux.NewRouter()
	s.AddHandlers(router)
	w := httptest.NewRecorder()

	return ctx, desc, s, router, w
}

func newAuthorizedRequest(method, target string, body io.Reader) *http.Request {
	ret := httptest.NewRequest(method, target, body)
	ret.Header.Add(authproxy.WebAuthRoleHeaderName, string(roles.Editor))
	return ret
}

func TestMachineToggleModeHandler_Success(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	changeSinkMock := s.pubsubChangeSink.(*changeSinkMocks.Sink)
	changeSinkMock.On("Send", testutils.AnyContext, machineID).Return(nil).Times(2)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Update", testutils.AnyContext, machineID, mock.Anything).Return(nil)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/toggle_mode/%s", machineID), nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestToggleMode_ChangesModeAndAddsAnnotation(t *testing.T) {
	ctx, desc, _, _, _ := setupForTest(t)
	desc.MaintenanceMode = ""

	retDesc := toggleMode(ctx, testUser, desc)

	expected := machine.Annotation{
		Message:   `Enabled Maintenance Mode`,
		User:      "somebody@example.org",
		Timestamp: fakeTime,
	}
	require.Equal(t, expected, retDesc.Annotation)
	require.Equal(t, "somebody@example.org 2021-09-01T02:03:04Z", retDesc.MaintenanceMode)
}

func TestMachineToggleModeHandler_FailOnMissingID(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", "/_/machine/toggle_mode/", nil)

	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestMachineTogglePowerCycleHandler_Success(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Update", testutils.AnyContext, machineID, mock.Anything).Return(nil)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/toggle_powercycle/%s", machineID), nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestMachineTogglePowerCycleHandler_FailOnMissingID(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", "/_/machine/toggle_powercycle/", nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestTogglePowerCycle_SetsFlagAndAddsAnnotation(t *testing.T) {
	ctx, desc, _, _, _ := setupForTest(t)

	retDesc := togglePowerCycle(ctx, machineID, testUser, desc)

	expected := machine.Annotation{
		Message:   `Requested powercycle for "skia-rpi2-rack4-shelf1-001"`,
		User:      "somebody@example.org",
		Timestamp: fakeTime,
	}
	require.Equal(t, expected, retDesc.Annotation)
	require.True(t, retDesc.PowerCycle)
}

func TestMachineSetAttachedDeviceHandler_Success(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	body := testutils.MarshalJSONReader(t,
		rpc.SetAttachedDevice{
			AttachedDevice: machine.AttachedDeviceIOS,
		})
	storeMock.On("Update", testutils.AnyContext, machineID, mock.Anything).Return(nil)
	changeSinkMock := s.pubsubChangeSink.(*changeSinkMocks.Sink)
	changeSinkMock.On("Send", testutils.AnyContext, machineID).Return(nil).Times(2)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/set_attached_device/%s", machineID), body)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestMachineSetAttachedDevice_FailOnInvalidJSON(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/set_attached_device/%s", machineID), strings.NewReader("not valid json"))

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMachineSetAttachedDevice_FailOnMissingID(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", "/_/machine/set_attached_device/", nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSetAttachedDevice_UpdatesAttachedDeviceField(t *testing.T) {
	_, desc, _, _, _ := setupForTest(t)
	retDesc := setAttachedDevice(machine.AttachedDeviceIOS, desc)
	require.Equal(t, machine.AttachedDeviceIOS, retDesc.AttachedDevice)
}

func TestMachineRemoveDeviceHandler_Success(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	body := testutils.MarshalJSONReader(t,
		rpc.SetAttachedDevice{
			AttachedDevice: machine.AttachedDeviceIOS,
		})
	storeMock.On("Update", testutils.AnyContext, machineID, mock.Anything).Return(nil)
	changeSinkMock := s.pubsubChangeSink.(*changeSinkMocks.Sink)
	changeSinkMock.On("Send", testutils.AnyContext, machineID).Return(nil).Times(2)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/remove_device/%s", machineID), body)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestMachineRemoveDevice_FailOnMissingID(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", "/_/machine/remove_device/", nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestRemoveDevice(t *testing.T) {
	ctx, desc, _, _, _ := setupForTest(t)
	retDesc := removeDevice(ctx, machineID, testUser, desc)

	expected := machine.Annotation{
		Message:   `Requested device removal of "skia-rpi2-rack4-shelf1-001"`,
		User:      "somebody@example.org",
		Timestamp: fakeTime,
	}

	require.Equal(t, expected, retDesc.Annotation)
	require.Empty(t, retDesc.Dimensions)
	require.Empty(t, retDesc.Temperature)
	require.Empty(t, retDesc.SuppliedDimensions)
}

func TestMachineDeleteMachineHandler_Success(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Delete", testutils.AnyContext, machineID).Return(nil)
	changeSinkMock := s.pubsubChangeSink.(*changeSinkMocks.Sink)
	changeSinkMock.On("Send", testutils.AnyContext, machineID).Return(nil).Times(2)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/delete_machine/%s", machineID), nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestMachineDeleteMachineHandler_DeleteFails_ReturnsStatusBadRequest(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Delete", testutils.AnyContext, machineID).Return(errFake)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/delete_machine/%s", machineID), nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMachineDeleteMachineHandler_MissingMachineID_ReturnsStatusNotFound(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", "/_/machine/delete_machine/", nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestMachineSetNoteHandler_Success(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	body := testutils.MarshalJSONReader(t,
		rpc.SetNoteRequest{
			Message: "this is a message",
		})
	storeMock.On("Update", testutils.AnyContext, machineID, mock.Anything).Return(nil)
	changeSinkMock := s.pubsubChangeSink.(*changeSinkMocks.Sink)
	changeSinkMock.On("Send", testutils.AnyContext, machineID).Return(nil).Times(2)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/set_note/%s", machineID), body)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestMachineSetNoteHandler_ReceivesInvalidJSON_ReturnsStatusBadRequest(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/set_note/%s", machineID), strings.NewReader("this is not valid json"))

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMachineSetNoteHandler_MissingID_ReturnsStatusNotFound(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", "/_/machine/set_note/", strings.NewReader("this is not valid json"))

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestSetNote_AddsAnnotationWithTimestamp(t *testing.T) {

	ctx, desc, _, _, _ := setupForTest(t)

	const message = "This is a test message."
	note := rpc.SetNoteRequest{
		Message: message,
	}

	retDesc := setNote(ctx, testUser, note, desc)
	expected := machine.Annotation{
		Message:   message,
		User:      "somebody@example.org",
		Timestamp: fakeTime,
	}
	require.Equal(t, expected, retDesc.Note)
}

func TestMachineSupplyChromeOSInfoHandler_Success(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	body := testutils.MarshalJSONReader(t,
		rpc.SupplyChromeOSRequest{
			SSHUserIP:          sshUserIP2,
			SuppliedDimensions: suppliedDimensions2,
		})
	storeMock.On("Update", testutils.AnyContext, machineID, mock.Anything).Return(nil)
	changeSinkMock := s.pubsubChangeSink.(*changeSinkMocks.Sink)
	changeSinkMock.On("Send", testutils.AnyContext, machineID).Return(nil).Times(2)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/supply_chromeos/%s", machineID), body)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestMachineSupplyChromeOSInfoHandler_SSHUserIPMissing_ReturnsStatusBadRequest(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	body := testutils.MarshalJSONReader(t,
		rpc.SupplyChromeOSRequest{
			SSHUserIP:          "",
			SuppliedDimensions: suppliedDimensions2,
		})
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/supply_chromeos/%s", machineID), body)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestMachineSupplyChromeOSInfoHandler_MissingMachineID_ReturnsStatusNotFound(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", "/_/machine/supply_chromeos/", nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}
func TestSetChromeOSInfo_SuppliedDimensionsChange(t *testing.T) {
	ctx, desc, _, _, _ := setupForTest(t)

	req := rpc.SupplyChromeOSRequest{
		SSHUserIP:          sshUserIP2,
		SuppliedDimensions: suppliedDimensions2,
	}
	retDesc := setChromeOSInfo(ctx, req, desc)

	require.Equal(t, sshUserIP2, retDesc.SSHUserIP)
	require.Equal(t, suppliedDimensions2, retDesc.SuppliedDimensions)
	require.Equal(t, fakeTime, retDesc.LastUpdated)
}

func TestApiMachineDescriptionHandler_GoodMachineID_ReturnsFrontendDescription(t *testing.T) {
	ctx, desc, s, router, w := setupForTest(t)

	storeMock := s.store.(*mocks.Store)
	storeMock.On("Get", testutils.AnyContext, machineID).Return(desc, nil)

	r := newAuthorizedRequest("GET", fmt.Sprintf("/json/v1/machine/description/%s", machineID), nil)
	r = r.WithContext(ctx)

	// Make the request.
	router.ServeHTTP(w, r)

	var actual rpc.FrontendDescription
	err := json.Unmarshal(w.Body.Bytes(), &actual)
	require.NoError(t, err)
	assert.Equal(t, rpc.ToFrontendDescription(desc), actual)
}

func TestApiMachineDescriptionHandler_StoreGetFails_ReturnsInternalServerError(t *testing.T) {
	_, desc, s, router, w := setupForTest(t)

	storeMock := s.store.(*mocks.Store)
	storeMock.On("Get", testutils.AnyContext, machineID).Return(desc, errFake)

	r := newAuthorizedRequest("GET", fmt.Sprintf("/json/v1/machine/description/%s", machineID), nil)

	// Make the request.
	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestApiMachineDescriptionHandler_NoMachineIDSupplied_ReturnsNotFound(t *testing.T) {
	_, _, _, router, w := setupForTest(t)

	r := newAuthorizedRequest("GET", "/json/v1/machine/description/", nil)

	// Make the request.
	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestApiPowerCycleListHandler_NoMachinesNeedPowerCycling_ReturnsEmptyList(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)

	machines := []string{}
	storeMock.On("ListPowerCycle", testutils.AnyContext).Return(machines, nil)

	r := newAuthorizedRequest("GET", "/json/v1/powercycle/list", nil)

	// Make the request.
	router.ServeHTTP(w, r)

	var actual rpc.ListPowerCycleResponse
	err := json.Unmarshal(w.Body.Bytes(), &actual)
	require.NoError(t, err)
	assert.Equal(t, rpc.ToListPowerCycleResponse(machines), actual)
}

func TestApiPowerCycleListHandler_OneMachineNeedsPowerCycling_ReturnsOneMachineInList(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)

	machines := []string{machineID}
	storeMock.On("ListPowerCycle", testutils.AnyContext).Return(machines, nil)

	r := newAuthorizedRequest("GET", "/json/v1/powercycle/list", nil)

	// Make the request.
	router.ServeHTTP(w, r)

	var actual rpc.ListPowerCycleResponse
	err := json.Unmarshal(w.Body.Bytes(), &actual)
	require.NoError(t, err)
	assert.Equal(t, rpc.ToListPowerCycleResponse(machines), actual)
}

func TestApiPowerCycleListHandler_ListPowerCycleReturnsError_ReturnsInternalServerError(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)

	storeMock.On("ListPowerCycle", testutils.AnyContext).Return(nil, errFake)

	r := newAuthorizedRequest("GET", "/json/v1/powercycle/list", nil)

	// Make the request.
	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestApiPowerCycleCompleteHandler_UpdateSucceeds_ReturnStatusOK(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Update", testutils.AnyContext, machineID, mock.AnythingOfType("store.UpdateCallback")).Return(nil)

	r := newAuthorizedRequest("POST", fmt.Sprintf("/json/v1/powercycle/complete/%s", machineID), nil)

	// Make the request.
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusOK, w.Code)
}

func TestApiPowerCycleCompleteHandler_UpdateFails_ReturnStatusInternalServerError(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Update", testutils.AnyContext, machineID, mock.AnythingOfType("store.UpdateCallback")).Return(errFake)

	r := newAuthorizedRequest("POST", fmt.Sprintf("/json/v1/powercycle/complete/%s", machineID), nil)

	// Make the request.
	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestSetPowerCycleFalse_PowerCycleIsTrue_PowerCycleBecomesFalse(t *testing.T) {
	ctx := context.Background()
	desc := machine.NewDescription(ctx)
	desc.PowerCycle = true
	desc = setPowerCycleFalse(desc)
	require.False(t, desc.PowerCycle)
}

func TestSetPowerCycleStateNotAvailable_PowerCycleStateIsAvailable_PowerCycleStateBecomesNotAvailable(t *testing.T) {
	ctx := context.Background()
	desc := machine.NewDescription(ctx)
	desc.PowerCycleState = machine.Available
	desc = setPowerCycleState(machine.NotAvailable, desc)
	require.Equal(t, machine.NotAvailable, desc.PowerCycleState)
}

var validUpdatePowerCycleStateRequest = rpc.UpdatePowerCycleStateRequest{
	Machines: []rpc.PowerCycleStateForMachine{
		{
			MachineID:       machineID,
			PowerCycleState: machine.Available,
		},
	},
}

func TestApiPowerCycleStateUpdateHandler_MachineDoesNotExist_TheMachineIsSkippedAndUpdateIsNeverCalled(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Get", testutils.AnyContext, machineID).Return(machine.Description{}, errFake)

	r := newAuthorizedRequest("POST", rpc.PowerCycleStateUpdateURL, testutils.MarshalJSONReader(t, validUpdatePowerCycleStateRequest))

	// Make the request.
	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestApiPowerCycleStateUpdateHandler_UpdateFails_ReturnStatusInternalServerError(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Update", testutils.AnyContext, machineID, mock.AnythingOfType("store.UpdateCallback")).Return(errFake)
	storeMock.On("Get", testutils.AnyContext, machineID).Return(machine.Description{}, nil)

	r := newAuthorizedRequest("POST", rpc.PowerCycleStateUpdateURL, testutils.MarshalJSONReader(t, validUpdatePowerCycleStateRequest))

	// Make the request.
	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestApiPowerCycleStateUpdateHandler_InvalidJSON_ReturnStatusBadRequest(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", rpc.PowerCycleStateUpdateURL, strings.NewReader("this isn't valid json"))

	// Make the request.
	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestApiPowerCycleStateUpdateHandler_ValidRequest_DescriptionsAreSuccessfullyUpdated(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Update", testutils.AnyContext, machineID, mock.AnythingOfType("store.UpdateCallback")).Return(nil).Once()
	storeMock.On("Get", testutils.AnyContext, machineID).Return(machine.Description{}, nil)
	r := newAuthorizedRequest("POST", rpc.PowerCycleStateUpdateURL, testutils.MarshalJSONReader(t, validUpdatePowerCycleStateRequest))

	// Make the request.
	router.ServeHTTP(w, r)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestMachineClearQuarantineHandler_Success(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Update", testutils.AnyContext, machineID, mock.Anything).Return(nil)
	changeSinkMock := s.pubsubChangeSink.(*changeSinkMocks.Sink)
	changeSinkMock.On("Send", testutils.AnyContext, machineID).Return(nil).Times(2)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/clear_quarantined/%s", machineID), nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code)
}

func TestMachineClearQuarantineHandler_StoreReturnsError_ReturnsInternalServerError(t *testing.T) {
	_, _, s, router, w := setupForTest(t)
	storeMock := s.store.(*mocks.Store)
	storeMock.On("Update", testutils.AnyContext, machineID, mock.Anything).Return(errFake)
	r := newAuthorizedRequest("POST", fmt.Sprintf("/_/machine/clear_quarantined/%s", machineID), nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestMachineClearQuarantineHandler_MachineIDNotSupplied_ReturnsNotFound(t *testing.T) {
	_, _, _, router, w := setupForTest(t)
	r := newAuthorizedRequest("POST", "/_/machine/clear_quarantined/", nil)

	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusNotFound, w.Code)
}

func TestClearQuarantined(t *testing.T) {
	require.False(t, clearQuarantined(machine.Description{IsQuarantined: true}).IsQuarantined)
}

func TestServerListenMachineEvents_EventsArriveOnBothChannels_BothEventsAreProcessed(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)

	numUpdateCalled := 0
	store := mocks.NewStore(t)
	store.On("Update", testutils.AnyContext, machineID, mock.Anything).Times(2).Run(func(_ mock.Arguments) {
		// Only after both events have arrived should be cancel the context to
		// exit from listenMachineEvents.
		numUpdateCalled++
		if numUpdateCalled == 2 {
			cancel()
		}
	}).Return(nil)

	event := machine.NewEvent()
	event.Host.Name = machineID

	pubsubSourceCh := make(chan machine.Event, 1)
	httpSourceCh := make(chan machine.Event, 1)

	s := &server{
		pubsubSourceCh: pubsubSourceCh,
		httpSourceCh:   httpSourceCh,
		store:          store,
	}

	pubsubSourceCh <- event
	httpSourceCh <- event

	s.listenMachineEvents(ctx)
	// Test will timeout if listenMachineEvents doesn't return.
}
