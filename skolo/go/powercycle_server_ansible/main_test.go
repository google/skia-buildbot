package main

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/machine/go/machineserver/rpc"
	"go.skia.org/infra/skolo/go/powercycle"
	"go.skia.org/infra/skolo/go/powercycle/mocks"
)

func setupForTest(t *testing.T, cb http.HandlerFunc) (*url.URL, *bool, *http.Client) {
	unittest.SmallTest(t)
	t.Helper()
	called := false
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cb(w, r)
		called = true
	}))
	t.Cleanup(func() {
		ts.Close()
	})
	u, err := url.Parse(ts.URL)
	require.NoError(t, err)

	httpClient := httputils.DefaultClientConfig().With2xxOnly().WithoutRetries().Client()
	return u, &called, httpClient
}

func TestSingleStep_MalformedMachineServerURL_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)
	err := singleStep(context.Background(), nil, "http://spaces in host names are invalid.com/", []powercycle.DeviceID{}, nil)
	require.Error(t, err)
}

func TestSingleStep_PowerCycleListEndpointReturnsError_ReturnsError(t *testing.T) {
	u, called, client := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, rpc.PowerCycleListURL, r.URL.Path)
		http.Error(w, "error", http.StatusInternalServerError)
	})
	err := singleStep(context.Background(), client, u.String(), []powercycle.DeviceID{}, nil)
	require.Error(t, err)
	require.True(t, *called)
}

func TestSingleStep_EmptyPowerCycleListReturnedFromEndpoint_Success(t *testing.T) {
	u, called, client := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, rpc.PowerCycleListURL, r.URL.Path)
		err := json.NewEncoder(w).Encode(rpc.ListPowerCycleResponse{})
		require.NoError(t, err)
	})
	err := singleStep(context.Background(), client, u.String(), []powercycle.DeviceID{}, nil)
	require.NoError(t, err)
	require.True(t, *called)
}

func TestSingleStep_PowerCycleListDoesNotContainAnyMachinesThatMatchTheDeviceIDs_Success(t *testing.T) {
	u, called, client := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, rpc.PowerCycleListURL, r.URL.Path)
		err := json.NewEncoder(w).Encode(rpc.ListPowerCycleResponse{"skia-rpi2-rack4-shelf1-001", "skia-rpi2-rack3-shelf2-002"})
		require.NoError(t, err)
	})
	err := singleStep(context.Background(), client, u.String(), []powercycle.DeviceID{}, nil) // deviceIDs is empty, so none of the machine ids will match.
	require.NoError(t, err)
	require.True(t, *called)
}

func TestSingleStep_PowerCycleListContainsAMatchingDeviceID_Success(t *testing.T) {
	const matchingMachineID = "skia-rpi2-rack4-shelf1-001"

	// Setup a callback that will handle both URLs that singleStep will make, first the call
	// to get the list of machines to powercycle, and then the list of methods to expect
	// at those URLs.
	expectedURLs := []string{
		rpc.PowerCycleListURL,
		urlExpansionRegex.ReplaceAllLiteralString(rpc.PowerCycleCompleteURL, matchingMachineID),
	}
	expectedMethods := []string{
		"GET",
		"POST",
	}
	currentRequest := 0 // Index into expectedMethods and expectedURLs.
	u, called, client := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, expectedURLs[currentRequest], r.URL.Path)
		require.Equal(t, expectedMethods[currentRequest], r.Method)
		if currentRequest == 0 {
			err := json.NewEncoder(w).Encode(rpc.ListPowerCycleResponse{matchingMachineID, "skia-rpi2-rack3-shelf2-002"})
			require.NoError(t, err)
		}
		currentRequest++
	})

	// Mock the powercycle.Controller to return success on PowerCycle().
	controllerMock := &mocks.Controller{}
	controllerMock.On("PowerCycle", testutils.AnyContext, powercycle.DeviceID(matchingMachineID), time.Duration(0)).Return(nil)

	err := singleStep(context.Background(), client, u.String(), []powercycle.DeviceID{matchingMachineID}, controllerMock)
	require.NoError(t, err)
	require.True(t, *called)
	controllerMock.AssertExpectations(t)
}

func TestSingleStep_PowerCycleControllerPowerCycleCallFails_ReturnsError(t *testing.T) {
	const matchingMachineID = "skia-rpi2-rack4-shelf1-001"
	u, called, client := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, rpc.PowerCycleListURL, r.URL.Path)
		err := json.NewEncoder(w).Encode(rpc.ListPowerCycleResponse{matchingMachineID, "skia-rpi2-rack3-shelf2-002"})
		require.NoError(t, err)
	})

	// Mock the powercycle.Controller to return an error on PowerCycle().
	myFakeError := errors.New("my fake error")
	controllerMock := &mocks.Controller{}
	controllerMock.On("PowerCycle", testutils.AnyContext, powercycle.DeviceID(matchingMachineID), time.Duration(0)).Return(myFakeError)

	err := singleStep(context.Background(), client, u.String(), []powercycle.DeviceID{matchingMachineID}, controllerMock)
	require.Error(t, err)
	require.True(t, *called)
	controllerMock.AssertExpectations(t)
}

func TestSingleStep_PowerCycleCompleteCallFails_ReturnsError(t *testing.T) {
	const matchingMachineID = "skia-rpi2-rack4-shelf1-001"

	// Setup a callback that will handle both URLs that singleStep will make, first the call
	// to get the list of machines to powercycle, and then the list of methods to expect
	// at those URLs.
	expectedURLs := []string{
		rpc.PowerCycleListURL,
		urlExpansionRegex.ReplaceAllLiteralString(rpc.PowerCycleCompleteURL, matchingMachineID),
	}
	expectedMethods := []string{
		"GET",
		"POST",
	}
	currentRequest := 0 // Index into expectedMethods and expectedURLs.
	u, called, client := setupForTest(t, func(w http.ResponseWriter, r *http.Request) {
		require.Equal(t, expectedURLs[currentRequest], r.URL.Path)
		require.Equal(t, expectedMethods[currentRequest], r.Method)
		if currentRequest == 0 {
			err := json.NewEncoder(w).Encode(rpc.ListPowerCycleResponse{matchingMachineID, "skia-rpi2-rack3-shelf2-002"})
			require.NoError(t, err)
		} else {
			http.Error(w, "failed to update machine server database", http.StatusInternalServerError)
		}
		currentRequest++
	})

	// Mock the powercycle.Controller to return success on PowerCycle().
	controllerMock := &mocks.Controller{}
	controllerMock.On("PowerCycle", testutils.AnyContext, powercycle.DeviceID(matchingMachineID), time.Duration(0)).Return(nil)

	err := singleStep(context.Background(), client, u.String(), []powercycle.DeviceID{matchingMachineID}, controllerMock)
	require.Error(t, err)
	require.True(t, *called)
	controllerMock.AssertExpectations(t)
}
