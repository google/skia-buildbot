package gatherer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/prometheus/common/model"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/promalertsclient"
	skswarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
)

var cycleTests = map[string]func(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, bd *mockBotDecider){
	"NoBots":              testNoBotsCycle,
	"NoAlertingBots":      testNoAlertingBots,
	"OneMissingBot":       testOneMissingBot,
	"ThreeMissingDevices": testThreeMissingDevices,
}

// To cut down on the boilerplate of setting up the various mocks and asserting expectations, we (ab)use Go's ability to have "subtests". This allows us to make our test functions take the mocks as input and for us to assert the expectations after it completes. The asserting of the expectations after the test is why we cannot easily have just a setup function that all the tests call to make the mocks. Additionally, the use of package level variables for the mocks is not thread-safe if tests are run in parallel.
// https://golang.org/pkg/testing/#hdr-Subtests_and_Sub_benchmarks
func TestCycle(t *testing.T) {
	for name, test := range cycleTests {
		t.Run(name, func(t *testing.T) {
			testutils.SmallTest(t)
			// mi = "mock internal" client
			mi := skswarming.NewMockApiClient()
			// me = "mock external" client
			me := skswarming.NewMockApiClient()
			// ma = "mock alerts" client
			ma := promalertsclient.NewMockClient()
			bd := &mockBotDecider{}
			defer mi.AssertExpectations(t)
			defer me.AssertExpectations(t)
			defer ma.AssertExpectations(t)
			defer bd.AssertExpectations(t)
			test(t, mi, me, ma, bd)
		})
	}
}

func testNoBotsCycle(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, bd *mockBotDecider) {
	mi.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	me.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC))

	// There is a bit of whitebox testing here.  We can't mock out a call to GetAlerts if it won't be called.
	g := New(me, mi, ma, bd).(*gatherer)
	g.cycle()

	bots, err := g.GetDownBots()
	assert.NoError(t, err)
	assert.Empty(t, bots, "There should be no bots to reboot, because swarming doesn't detect any are down.")
}

func testNoAlertingBots(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, bd *mockBotDecider) {
	mi.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		jsonToBotInfo(MISSING_DEVICE_JSON, "skia-rpi-046"),
	}
	me.On("ListDownBots", mock.AnythingOfType("string")).Return(b, nil).Times(len(skswarming.POOLS_PUBLIC))

	ma.On("GetAlerts", mock.AnythingOfType("func(model.Alert) bool")).Return([]model.Alert{}, nil).Once()

	g := New(me, mi, ma, bd).(*gatherer)
	g.cycle()

	bots, err := g.GetDownBots()
	assert.NoError(t, err)
	assert.Empty(t, bots, "There should be no bots to reboot, because alerts says none are down.")
}

func testOneMissingBot(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, bd *mockBotDecider) {
	mi.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		jsonToBotInfo(DEAD_BOT, "skia-rpi-046"),
		jsonToBotInfo(MISSING_DEVICE_JSON, "skia-rpi-001"),
	}
	me.On("ListDownBots", mock.AnythingOfType("string")).Return(b, nil).Once()
	// return nothing for rest of the pools
	me.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC) - 1)

	ma.On("GetAlerts", mock.AnythingOfType("func(model.Alert) bool")).Return([]model.Alert{
		mockAPIAlert(ALERT_BOT_MISSING, "skia-rpi-046"),
	}, nil).Once()

	bd.On("ShouldPowercycleBot", mock.Anything).Return(true)
	bd.On("GetBugURL", mock.Anything).Return("")

	g := New(me, mi, ma, bd).(*gatherer)
	g.cycle()

	bots, err := g.GetDownBots()
	assert.NoError(t, err)
	assert.Len(t, bots, 1, "There should be 1 bot to reboot.")
	assert.Equal(t, "skia-rpi-046", bots[0].BotID, "That bot should be skia-rpi-046")
	assert.Equal(t, STATUS_HOST_MISSING, bots[0].Status)
}

func testThreeMissingDevices(t *testing.T, mi, me *skswarming.MockApiClient, ma *promalertsclient.MockAPIClient, bd *mockBotDecider) {
	mi.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		jsonToBotInfo(MISSING_DEVICE_JSON, "skia-rpi-001"),
		jsonToBotInfo(MISSING_DEVICE_JSON, "skia-rpi-003"),
		jsonToBotInfo(USB_FAILURE_JSON, "skia-rpi-002"),
		jsonToBotInfo(USB_FAILURE_JSON, "skia-rpi-120"),
		jsonToBotInfo(TOO_HOT_JSON, "skia-rpi-121"),
		jsonToBotInfo(DEAD_BOT, "skia-vm-001"),
	}
	me.On("ListDownBots", mock.AnythingOfType("string")).Return(b, nil).Once()
	// return nothing for rest of the pools
	me.On("ListDownBots", mock.AnythingOfType("string")).Return([]*swarming.SwarmingRpcsBotInfo{}, nil).Times(len(skswarming.POOLS_PUBLIC) - 1)

	ma.On("GetAlerts", mock.AnythingOfType("func(model.Alert) bool")).Return([]model.Alert{
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-003"),
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-001"),
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-002"), // This one has a usb failure, which is sometimes fixed by a powercycle
		mockAPIAlert(ALERT_BOT_QUARANTINED, "skia-rpi-121"), // This one is too hot, and should not be offered for a reboot
	}, nil).Once()

	bd.On("ShouldPowercycleBot", mock.Anything).Return(false)
	bd.On("ShouldPowercycleDevice", mock.MatchedBy(func(bot *swarming.SwarmingRpcsBotInfo) bool {
		return bot.BotId == "skia-rpi-121"
	})).Return(false)
	bd.On("ShouldPowercycleDevice", mock.Anything).Return(true)
	bd.On("GetBugURL", mock.Anything).Return("")

	g := New(me, mi, ma, bd).(*gatherer)
	g.cycle()

	bots, err := g.GetDownBots()
	assert.NoError(t, err)
	assert.Len(t, bots, 3, "There should be 3 devices to reboot.")
	assert.Equal(t, "skia-rpi-001-device", bots[0].BotID, "These should be sorted alphabetically")
	assert.Equal(t, "skia-rpi-002-device", bots[1].BotID, "These should be sorted alphabetically")
	assert.Equal(t, "skia-rpi-003-device", bots[2].BotID, "These should be sorted alphabetically")
	assert.Equal(t, STATUS_DEVICE_MISSING, bots[0].Status)
	assert.Equal(t, STATUS_DEVICE_MISSING, bots[1].Status)
	assert.Equal(t, STATUS_DEVICE_MISSING, bots[2].Status)
}

func jsonToBotInfo(j, botId string) *swarming.SwarmingRpcsBotInfo {
	b := bytes.NewBufferString(j)
	var s swarming.SwarmingRpcsBotInfo
	d := json.NewDecoder(b)
	if err := d.Decode(&s); err != nil {
		fmt.Println("Error parsing json: %s", err)
		return nil
	}
	s.BotId = botId
	return &s
}

func mockAPIAlert(alertname, bot string) model.Alert {
	a := model.Alert{
		Labels: model.LabelSet{},
	}
	a.Labels["alertname"] = model.LabelValue(alertname)
	a.Labels["bot"] = model.LabelValue(bot)
	return a
}

// The following JSON was recieved from calls to
// https://chromium-swarm.appspot.com/_ah/api/swarming/v1/bot/[bot]/get
// when bots were displaying interesting behavior
var TOO_HOT_JSON = `{"authenticated_as": "bot:whitelisted-ip", "dimensions": [{"value": ["1"], "key": "android_devices"}, {"value": ["N", "NRD90M", "NRD90M_G930AUCS4BQC2"], "key": "device_os"}, {"value": ["heroqlteatt"], "key": "device_type"}, {"value": ["skia-rpi-035"], "key": "id"}, {"value": ["Android"], "key": "os"}, {"value": ["Skia"], "key": "pool"}, {"value": ["Device Missing"], "key": "quarantined"}], "task_id": "", "external_ip": "172.23.215.237", "is_dead": false, "quarantined": true, "deleted": false, "state": "{\"audio\":null,\"bot_group_cfg_version\":\"hash:d0797123288bd0\",\"cores\":[\"4\"],\"cost_usd_hour\":0.15238326280381945,\"cpu\":[\"armv7l\",\"armv7l-32\"],\"cwd\":\"/b/s\",\"devices\":{\"fb452058\":{\"battery\":{\"current\":null,\"health\":2,\"level\":100,\"power\":[\"USB\"],\"status\":2,\"temperature\":301,\"voltage\":4297},\"build\":{\"board.platform\":\"msm8996\",\"build.fingerprint\":\"samsung/heroqlteuc/heroqlteatt:7.0/NRD90M/G930AUCS4BQC2:user/release-keys\",\"build.id\":\"NRD90M\",\"build.version.sdk\":\"24\",\"product.board\":\"msm8996\",\"product.cpu.abi\":\"arm64-v8a\"},\"cpu\":{\"cur\":\"1228800\",\"governor\":null},\"disk\":{\"cache\":{\"free_mb\":986.5,\"size_mb\":991.89999999999998},\"data\":{\"free_mb\":22272.700000000001,\"size_mb\":23878.900000000001},\"system\":{\"free_mb\":242.59999999999999,\"size_mb\":4687.3000000000002}},\"imei\":\"352325080500650\",\"ip\":[],\"max_uid\":null,\"mem\":{},\"other_packages\":[\"com.mobeam.barcodeService\",\"com.amazon.kindle\"],\"port_path\":\"1/12\",\"processes\":562,\"state\":\"too_hot\",\"temp\":{\"ac\":35.5,\"battery\":30.399999999999999,\"emmc_therm\":31.0,\"max77854-fuelgauge\":30.199999999999999,\"msm_therm\":46.0,\"pa_therm0\":41.0,\"pa_therm1\":37.0,\"pm8004_tz\":37.0,\"pm8994_tz\":42.325000000000003,\"tsens_tz_sensor0\":435.0,\"tsens_tz_sensor1\":448.0,\"tsens_tz_sensor10\":534.0,\"tsens_tz_sensor11\":509.0,\"tsens_tz_sensor12\":493.0,\"tsens_tz_sensor13\":462.0,\"tsens_tz_sensor14\":462.0,\"tsens_tz_sensor15\":462.0,\"tsens_tz_sensor16\":469.0,\"tsens_tz_sensor17\":469.0,\"tsens_tz_sensor18\":456.0,\"tsens_tz_sensor19\":456.0,\"tsens_tz_sensor2\":438.0,\"tsens_tz_sensor20\":475.0,\"tsens_tz_sensor3\":445.0,\"tsens_tz_sensor4\":445.0,\"tsens_tz_sensor5\":467.0,\"tsens_tz_sensor6\":474.0,\"tsens_tz_sensor7\":506.0,\"tsens_tz_sensor8\":547.0,\"tsens_tz_sensor9\":557.0,\"xo_therm_buf\":42.0},\"uptime\":64.519999999999996}},\"disks\":{\"/b\":{\"free_mb\":4298.0,\"size_mb\":26746.5},\"/boot\":{\"free_mb\":40.399999999999999,\"size_mb\":59.899999999999999},\"/home/chrome-bot\":{\"free_mb\":987.20000000000005,\"size_mb\":988.89999999999998},\"/tmp\":{\"free_mb\":974.60000000000002,\"size_mb\":975.89999999999998},\"/var\":{\"free_mb\":769.79999999999995,\"size_mb\":975.89999999999998}},\"gpu\":[\"none\"],\"hostname\":\"skia-rpi-035\",\"ip\":\"192.168.1.135\",\"locale\":\"Unknown\",\"machine_type\":[\"n1-highcpu-4\"],\"nb_files_in_temp\":7,\"periodic_reboot_secs\":43200,\"pid\":572,\"quarantined\":\"No available devices.\",\"ram\":926,\"running_time\":10586,\"sleep_streak\":2,\"started_ts\":1495099895,\"temp\":{\"thermal_zone0\":44.006999999999998},\"uptime\":10624,\"user\":\"chrome-bot\"}", "version": "7139d890ad38500480cd70d61a238f829bedcb01a08f0d2909d048dd23f2098a", "first_seen_ts": "2016-09-09T20:16:24.496470", "last_seen_ts": "2017-05-18T12:28:19.748440", "bot_id": "skia-rpi-035"}`

var USB_FAILURE_JSON = `{"authenticated_as": "bot:whitelisted-ip", "dimensions": [{"value": ["4"], "key": "cores"}, {"value": ["arm", "arm-32", "armv7l", "armv7l-32"], "key": "cpu"}, {"value": ["none"], "key": "gpu"}, {"value": ["skia-rpi-063"], "key": "id"}, {"value": ["n1-highcpu-4"], "key": "machine_type"}, {"value": ["Android"], "key": "os"}, {"value": ["Skia"], "key": "pool"}], "task_id": "", "external_ip": "172.23.215.237", "is_dead": false, "quarantined": true, "deleted": false, "state": "{\"audio\":null,\"bot_group_cfg_version\":\"hash:d0797123288bd0\",\"cores\":[\"4\"],\"cost_usd_hour\":0.15238277452256943,\"cpu\":[\"armv7l\",\"armv7l-32\"],\"cwd\":\"/b/s\",\"devices\":{\"6121001202\":{\"state\":\"usb_failure\"}},\"disks\":{\"/b\":{\"free_mb\":4286.3000000000002,\"size_mb\":26746.5},\"/boot\":{\"free_mb\":40.399999999999999,\"size_mb\":59.899999999999999},\"/home/chrome-bot\":{\"free_mb\":987.0,\"size_mb\":988.89999999999998},\"/tmp\":{\"free_mb\":974.60000000000002,\"size_mb\":975.89999999999998},\"/var\":{\"free_mb\":789.39999999999998,\"size_mb\":975.89999999999998}},\"gpu\":[\"none\"],\"hostname\":\"skia-rpi-063\",\"ip\":\"192.168.1.163\",\"locale\":\"Unknown\",\"machine_type\":[\"n1-highcpu-4\"],\"nb_files_in_temp\":7,\"periodic_reboot_secs\":43200,\"pid\":571,\"quarantined\":\"No available devices.\",\"ram\":926,\"running_time\":69110,\"sleep_streak\":424,\"started_ts\":1495126730,\"temp\":{\"thermal_zone0\":44.006999999999998},\"uptime\":69149,\"user\":\"chrome-bot\"}", "version": "7139d890ad38500480cd70d61a238f829bedcb01a08f0d2909d048dd23f2098a", "first_seen_ts": "2016-09-09T20:19:43.918890", "last_seen_ts": "2017-05-19T12:10:39.821360", "bot_id": "skia-rpi-063"}`

var MISSING_DEVICE_JSON = `{"authenticated_as": "bot:whitelisted-ip", "dimensions": [{"value": ["1"], "key": "android_devices"}, {"value": ["N", "NMF26Q"], "key": "device_os"}, {"value": ["sailfish"], "key": "device_type"}, {"value": ["skia-rpi-046"], "key": "id"}, {"value": ["Android"], "key": "os"}, {"value": ["Skia"], "key": "pool"}, {"value": ["Device Missing"], "key": "quarantined"}], "task_id": "", "external_ip": "172.23.215.237", "is_dead": false, "quarantined": true, "deleted": false, "state": "{\"audio\":null,\"bot_group_cfg_version\":\"hash:d0797123288bd0\",\"cost_usd_hour\":0.15337884114583333,\"cpu\":\"BCM2709\",\"cwd\":\"/b/s\",\"disks\":{\"/b\":{\"free_mb\":22441.700000000001,\"size_mb\":26746.400000000001},\"/boot\":{\"free_mb\":40.399999999999999,\"size_mb\":59.899999999999999},\"/home/chrome-bot\":{\"free_mb\":986.39999999999998,\"size_mb\":988.89999999999998},\"/tmp\":{\"free_mb\":973.39999999999998,\"size_mb\":975.89999999999998},\"/var\":{\"free_mb\":969.89999999999998,\"size_mb\":975.89999999999998}},\"gpu\":null,\"hostname\":\"skia-rpi-046\",\"ip\":\"192.168.1.146\",\"locale\":\"Unknown\",\"nb_files_in_temp\":5,\"periodic_reboot_secs\":43200,\"pid\":575,\"quarantined\":\"No available Android devices.\",\"ram\":926,\"running_time\":84013,\"sleep_streak\":1210,\"started_ts\":1495026223,\"temp\":{\"thermal_zone0\":47.774000000000001},\"uptime\":84084,\"user\":\"chrome-bot\"}", "version": "7139d890ad38500480cd70d61a238f829bedcb01a08f0d2909d048dd23f2098a", "first_seen_ts": "2016-09-12T15:30:09.375170", "last_seen_ts": "2017-05-18T12:23:56.969260", "bot_id": "skia-rpi-046"}`

var DEAD_BOT = `{"authenticated_as": "bot:whitelisted-ip", "dimensions": [{"value": ["1"], "key": "android_devices"}, {"value": ["N", "NRD90M", "NRD90M_G930FXXU1DQAS"], "key": "device_os"}, {"value": ["herolte"], "key": "device_type"}, {"value": ["skia-rpi-113"], "key": "id"}, {"value": ["Android"], "key": "os"}, {"value": ["Skia"], "key": "pool"}], "task_id": "", "external_ip": "172.23.215.237", "is_dead": true, "quarantined": false, "deleted": false, "state": "{\"audio\":null,\"bot_group_cfg_version\":\"hash:41e62eb67c5de6\",\"cores\":[\"4\"],\"cost_usd_hour\":0.15240563693576389,\"cpu\":[\"armv7l\",\"armv7l-32\"],\"cwd\":\"/b/s\",\"devices\":{\"ad0c1603b00f6e7324\":{\"battery\":{\"current\":null,\"health\":2,\"level\":100,\"power\":[\"USB\"],\"status\":5,\"temperature\":278,\"voltage\":4327},\"build\":{\"board.platform\":\"exynos5\",\"build.fingerprint\":\"samsung/heroltexx/herolte:7.0/NRD90M/G930FXXU1DQAS:user/release-keys\",\"build.id\":\"NRD90M\",\"build.version.sdk\":\"24\",\"product.board\":\"universal8890\",\"product.cpu.abi\":\"arm64-v8a\"},\"cpu\":{\"cur\":\"442000\",\"governor\":\"interactive\"},\"disk\":{\"cache\":{\"free_mb\":190.40000000000001,\"size_mb\":192.80000000000001},\"data\":{\"free_mb\":23520.900000000001,\"size_mb\":25321.299999999999},\"system\":{\"free_mb\":177.30000000000001,\"size_mb\":4199.1000000000004}},\"imei\":\"358436072849767\",\"ip\":[],\"max_uid\":null,\"mem\":{},\"other_packages\":[\"com.mobeam.barcodeService\",\"com.samsung.android.video\",\"com.sec.enterprise.knox.shareddevice.keyguard\"],\"port_path\":\"1/6\",\"processes\":322,\"state\":\"available\",\"temp\":{\"ac\":28.5,\"battery\":27.800000000000001,\"max77854-fuelgauge\":27.699999999999999,\"therm_zone0\":30.0,\"therm_zone1\":30.0,\"therm_zone2\":29.0,\"therm_zone3\":30.0,\"therm_zone4\":29.0},\"uptime\":2908.5500000000002}},\"disks\":{\"/b\":{\"free_mb\":4551.8999999999996,\"size_mb\":26746.5},\"/boot\":{\"free_mb\":40.399999999999999,\"size_mb\":59.899999999999999},\"/home/chrome-bot\":{\"free_mb\":987.60000000000002,\"size_mb\":988.89999999999998},\"/tmp\":{\"free_mb\":974.60000000000002,\"size_mb\":975.89999999999998},\"/var\":{\"free_mb\":921.89999999999998,\"size_mb\":975.89999999999998}},\"gpu\":[\"none\"],\"hostname\":\"skia-rpi-113\",\"ip\":\"192.168.1.213\",\"locale\":\"Unknown\",\"machine_type\":[\"n1-highcpu-4\"],\"nb_files_in_temp\":7,\"periodic_reboot_secs\":43200,\"pid\":571,\"ram\":926,\"running_time\":249,\"sleep_streak\":9,\"started_ts\":1495635893,\"temp\":{\"thermal_zone0\":52.615000000000002},\"uptime\":31426,\"user\":\"chrome-bot\"}", "version": "078bc890301b65ef1f056e01ccbfa34229dd2b8297577f914f971adcd280f125", "first_seen_ts": "2017-03-13T16:43:49.380160", "last_seen_ts": "2017-05-24T14:29:13.580110", "bot_id": "skia-rpi-113"}`

type mockBotDecider struct {
	mock.Mock
}

func (m *mockBotDecider) ShouldPowercycleBot(bot *swarming.SwarmingRpcsBotInfo) bool {
	args := m.Called(bot)
	return args.Bool(0)
}

func (m *mockBotDecider) ShouldPowercycleDevice(bot *swarming.SwarmingRpcsBotInfo) bool {
	args := m.Called(bot)
	return args.Bool(0)
}

func (m *mockBotDecider) GetBugURL(bot *swarming.SwarmingRpcsBotInfo) string {
	args := m.Called(bot)
	return args.String(0)
}
