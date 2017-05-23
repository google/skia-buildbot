package gatherer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"testing"

	swarming "github.com/luci/luci-go/common/api/swarming/swarming/v1"
	"github.com/prometheus/alertmanager/dispatch"
	"github.com/stretchr/testify/mock"
	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/promalertsclient"
	skswarming "go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/testutils"
)

var emptyList = []*swarming.SwarmingRpcsBotInfo{}

func TestNoBotsCycle(t *testing.T) {
	testutils.SmallTest(t)

	// mi = "mock internal" client
	mi := skswarming.NewMockApiClient()
	// me = "mock external" client
	me := skswarming.NewMockApiClient()
	// ma = "mock alerts" client
	ma := promalertsclient.NewMockClient()
	defer mi.AssertExpectations(t)
	defer me.AssertExpectations(t)
	defer ma.AssertExpectations(t)

	mi.On("ListDownBots", mock.AnythingOfType("string")).Return(emptyList, nil).Times(len(skswarming.POOLS_PRIVATE))
	me.On("ListDownBots", mock.AnythingOfType("string")).Return(emptyList, nil).Times(len(skswarming.POOLS_PUBLIC))

	g := New(me, mi, ma).(*gatherer)
	g.cycle()

	bots, err := g.GetDownBots()
	assert.NoError(t, err)
	assert.Empty(t, bots, "There should be no bots to reboot, because swarming doesn't detect any are down.")
}

func TestNoAlertingBots(t *testing.T) {
	testutils.SmallTest(t)

	// mi = "mock internal" client
	mi := skswarming.NewMockApiClient()
	// me = "mock external" client
	me := skswarming.NewMockApiClient()
	// ma = "mock alerts" client
	ma := promalertsclient.NewMockClient()
	defer mi.AssertExpectations(t)
	defer me.AssertExpectations(t)
	defer ma.AssertExpectations(t)

	mi.On("ListDownBots", mock.AnythingOfType("string")).Return(emptyList, nil).Times(len(skswarming.POOLS_PRIVATE))
	b := []*swarming.SwarmingRpcsBotInfo{
		jsonToBotInfo(TOO_HOT_JSON),
	}
	me.On("ListDownBots", mock.AnythingOfType("string")).Return(b, nil).Times(len(skswarming.POOLS_PUBLIC))

	ma.On("GetAlerts", mock.AnythingOfType("func(dispatch.APIAlert) bool")).Return([]dispatch.APIAlert{}, nil).Once()

	g := New(me, mi, ma).(*gatherer)
	g.cycle()

	bots, err := g.GetDownBots()
	assert.NoError(t, err)
	assert.Empty(t, bots, "There should be no bots to reboot, because alerts says none are down.")
}

func jsonToBotInfo(j string) *swarming.SwarmingRpcsBotInfo {
	b := bytes.NewBufferString(j)
	var s swarming.SwarmingRpcsBotInfo
	d := json.NewDecoder(b)
	if err := d.Decode(&s); err != nil {
		fmt.Println("Error parsing json: %s", err)
		return nil
	}
	return &s
}

var TOO_HOT_JSON = `{"authenticated_as": "bot:whitelisted-ip", "dimensions": [{"value": ["1"], "key": "android_devices"}, {"value": ["N", "NRD90M", "NRD90M_G930AUCS4BQC2"], "key": "device_os"}, {"value": ["heroqlteatt"], "key": "device_type"}, {"value": ["skia-rpi-035"], "key": "id"}, {"value": ["Android"], "key": "os"}, {"value": ["Skia"], "key": "pool"}, {"value": ["Device Missing"], "key": "quarantined"}], "task_id": "", "external_ip": "172.23.215.237", "is_dead": false, "quarantined": true, "deleted": false, "state": "{\"audio\":null,\"bot_group_cfg_version\":\"hash:d0797123288bd0\",\"cores\":[\"4\"],\"cost_usd_hour\":0.15238326280381945,\"cpu\":[\"armv7l\",\"armv7l-32\"],\"cwd\":\"/b/s\",\"devices\":{\"fb452058\":{\"battery\":{\"current\":null,\"health\":2,\"level\":100,\"power\":[\"USB\"],\"status\":2,\"temperature\":301,\"voltage\":4297},\"build\":{\"board.platform\":\"msm8996\",\"build.fingerprint\":\"samsung/heroqlteuc/heroqlteatt:7.0/NRD90M/G930AUCS4BQC2:user/release-keys\",\"build.id\":\"NRD90M\",\"build.version.sdk\":\"24\",\"product.board\":\"msm8996\",\"product.cpu.abi\":\"arm64-v8a\"},\"cpu\":{\"cur\":\"1228800\",\"governor\":null},\"disk\":{\"cache\":{\"free_mb\":986.5,\"size_mb\":991.89999999999998},\"data\":{\"free_mb\":22272.700000000001,\"size_mb\":23878.900000000001},\"system\":{\"free_mb\":242.59999999999999,\"size_mb\":4687.3000000000002}},\"imei\":\"352325080500650\",\"ip\":[],\"max_uid\":null,\"mem\":{},\"other_packages\":[\"com.mobeam.barcodeService\",\"com.samsung.android.provider.filterprovider\",\"com.monotype.android.font.rosemary\",\"com.sec.android.app.DataCreate\",\"com.gd.mobicore.pa\",\"com.matchboxmobile.wisp\",\"com.sec.android.widgetapp.samsungapps\",\"com.sec.vsim.ericssonnsds.webapp\",\"com.samsung.android.app.galaxyfinder\",\"com.sec.location.nsflp2\",\"com.samsung.android.themestore\",\"com.sec.android.app.chromecustomizations\",\"com.samsung.android.app.aodservice\",\"com.sec.android.app.parser\",\"com.samsung.svoice.sync\",\"com.samsung.android.calendar\",\"com.drivemode\",\"com.osp.app.signin\",\"com.samsung.clipboardsaveservice\",\"com.sec.automation\",\"com.sec.android.app.clipvideo\",\"com.sec.android.devicehelp\",\"com.samsung.android.provider.shootingmodeprovider\",\"com.sec.android.app.wfdbroker\",\"com.att.android.digitallocker\",\"com.samsung.android.app.withtv\",\"com.skms.android.agent\",\"com.samsung.android.hmt.vrshell\",\"com.sec.android.app.safetyassurance\",\"com.samsung.android.incallui\",\"com.sec.factory.camera\",\"com.sec.vsimservice\",\"org.simalliance.openmobileapi.service\",\"com.sec.usbsettings\",\"com.samsung.android.easysetup\",\"com.sec.android.easyonehand\",\"com.sec.factory\",\"com.samsung.context.hwlogcollector\",\"com.cequint.ecid\",\"com.sec.android.wallpapercropper2\",\"com.directv.dvrscheduler\",\"com.samsung.android.radiobasedlocation\",\"com.sec.android.cover.ledcover\",\"com.amazon.mShop.android.install\",\"com.sec.android.easyMover.Agent\",\"com.samsung.ucs.agent.boot\",\"com.wsomacp\",\"com.samsung.faceservice\",\"com.sec.android.app.easylauncher\",\"com.samsung.knox.rcp.components\",\"com.monotype.android.font.foundation\",\"com.sec.android.widgetapp.easymodecontactswidget\",\"com.samsung.android.email.provider\",\"com.samsung.android.intelligenceservice2\",\"com.samsung.android.MtpApplication\",\"com.sec.android.app.factorykeystring\",\"com.sec.android.app.samsungapps\",\"com.sec.android.emergencymode.service\",\"com.qualcomm.qti.auth.fidocryptoservice\",\"com.sec.android.app.wlantest\",\"com.sec.android.app.billing\",\"com.samsung.android.app.selfmotionpanoramaviewer\",\"com.sec.epdgtestapp\",\"com.samsung.android.game.gamehome\",\"com.fullscreen.android\",\"com.sec.android.daemonapp\",\"com.samsung.android.slinkcloud\",\"com.sec.ims\",\"com.sec.sve\",\"com.sec.enterprise.knox.attestation\",\"com.dsi.ant.service.socket\",\"com.sec.android.AutoPreconfig\",\"com.ipsec.service\",\"com.samsung.android.SettingsReceiver\",\"com.sec.android.app.soundalive\",\"com.sec.android.providers.security\",\"com.sec.android.provider.badge\",\"com.samsung.android.securitylogagent\",\"com.samsung.android.app.watchmanager\",\"com.samsung.android.app.assistantmenu\",\"com.locationlabs.cni.att\",\"com.samsung.android.communicationservice\",\"com.samsung.SMT\",\"com.samsung.cmh\",\"com.samsung.vvm\",\"com.sec.android.ofviewer\",\"com.samsung.android.app.multiwindow\",\"com.samsung.hs20provider\",\"net.aetherpal.device\",\"com.sec.att.usagemanager3\",\"com.sec.tetheringprovision\",\"com.dsi.ant.sample.acquirechannels\",\"com.samsung.android.smartface\",\"com.samsung.android.messaging\",\"com.sec.knox.foldercontainer\",\"com.samsung.klmsagent\",\"com.samsung.android.app.memo\",\"com.sec.android.app.SecSetupWizard\",\"com.hancom.office.editor.hidden\",\"com.sec.android.app.hwmoduletest\",\"com.sec.bcservice\",\"com.sec.spen.flashannotate\",\"com.sec.modem.settings\",\"com.sec.android.app.sysscope\",\"com.samsung.fresco.logging\",\"com.samsung.android.providers.context\",\"com.sec.android.app.servicemodeapp\",\"com.sec.android.preloadinstaller\",\"com.sec.android.uibcvirtualsoftkey\",\"com.samsung.android.sdk.professionalaudio.utility.jammonitor\",\"com.sec.android.gallery3d\",\"com.sec.imsservice\",\"com.sec.app.TransmitPowerService\",\"com.samsung.android.app.colorblind\",\"com.samsung.android.hmt.vrsvc\",\"com.samsung.storyservice\",\"com.att.myWireless\",\"com.sec.android.app.dictionary\",\"com.samsung.knox.securefolder\",\"com.samsung.android.app.talkback\",\"com.samsung.android.game.gametools\",\"com.asurion.android.mobilerecovery.att\",\"com.sec.android.gallery3d.panorama360view\",\"com.sec.smartcard.manager\",\"com.samsung.android.authservice\",\"com.sec.android.Preconfig\",\"com.qti.xdivert\",\"com.samsung.app.slowmotion\",\"com.samsung.android.smartcallprovider\",\"com.sec.enterprise.mdm.vpn\",\"com.directv.promo.shade\",\"com.samsung.android.weather\",\"com.dsi.ant.plugins.antplus\",\"com.samsung.android.personalpage.service\",\"com.samsung.advp.imssettings\",\"com.samsung.android.location\",\"com.sec.android.inputmethod\",\"com.samsung.android.app.advsounddetector\",\"com.sec.android.app.clockpackage\",\"com.sec.android.RilServiceModeApp\",\"com.sec.android.app.simsettingmgr\",\"com.samsung.android.app.mirrorlink\",\"com.qualcomm.qti.simsettings\",\"com.samsung.android.app.vrsetupwizardstub\",\"com.sec.imslogger\",\"com.sec.android.fido.uaf.client\",\"com.samsung.android.clipboarduiservice\",\"com.samsung.android.asksmanager\",\"com.samsung.android.themecenter\",\"com.samsung.android.sm\",\"com.samsung.android.controltv\",\"com.sec.android.diagmonagent\",\"com.trustonic.tuiservice\",\"org.codeaurora.btmultisim\",\"com.sec.spp.push\",\"com.dsi.ant.server\",\"com.sec.android.app.myfiles\",\"com.samsung.android.nsdsvowifi\",\"com.samsung.android.allshare.service.fileshare\",\"com.synchronoss.dcs.att.r2g\",\"com.sec.android.mimage.photoretouching\",\"com.sec.android.app.launcher\",\"com.samsung.android.universalswitch\",\"com.samsung.helphub\",\"com.sec.android.app.apex\",\"com.sec.android.app.sns3\",\"flipboard.boxer.app\",\"com.samsung.android.app.filterinstaller\",\"com.sec.android.providers.tasks\",\"com.samsung.android.authfw\",\"com.ubercab\",\"com.sec.android.app.sbrowser\",\"com.monotype.android.font.chococooky\",\"com.sec.android.service.health\",\"com.samsung.safetyinformation\",\"com.facebook.katana\",\"com.samsung.app.highlightplayer\",\"com.samsung.enhanceservice\",\"com.sec.android.app.ringtoneBR\",\"com.sec.android.app.vepreload\",\"com.sem.factoryapp\",\"com.samsung.android.keyguardwallpaperupdator\",\"com.samsung.android.app.accesscontrol\",\"com.samsung.android.beaconmanager\",\"com.sec.enterprise.mdm.services.simpin\",\"com.amazon.mShop.android\",\"com.facebook.system\",\"com.samsung.android.oneconnect\",\"com.sec.android.app.popupcalculator\",\"com.sec.android.soagent\",\"com.samsung.android.fmm\",\"com.samsung.android.mdm\",\"com.sec.android.fido.uaf.asm\",\"com.sec.android.app.shealth\",\"com.ws.dm\",\"com.sec.phone\",\"com.samsung.android.app.scrollcapture\",\"com.samsung.android.framework.res\",\"com.sec.knox.knoxsetupwizardclient\",\"com.samsung.android.app.interactivepanoramaviewer\",\"com.samsung.android.samsungpass\",\"com.samsung.android.scloud\",\"com.samsung.android.app.soundpicker\",\"com.sec.app.RilErrorNotifier\",\"com.samsung.android.spayfw\",\"com.samsung.app.newtrim\",\"com.samsung.android.lool\",\"com.samsung.android.spay\",\"com.sec.android.app.bluetoothtest\",\"com.samsung.android.sm.policy\",\"com.sec.android.emergencylauncher\",\"com.sec.hearingadjust\",\"com.qualcomm.location\",\"com.samsung.android.dlp.service\",\"com.samsung.android.bluelightfilter\",\"com.samsung.android.bbc.bbcagent\",\"com.samsung.android.voicewakeup\",\"com.sec.android.splitsound\",\"com.wssnps\",\"com.samsung.android.app.watchmanagerstub\",\"com.samsung.android.svcagent\",\"com.policydm\",\"com.samsung.android.sdk.professionalaudio.app.audioconnectionservice\",\"com.samsung.android.networkdiagnostic\",\"com.enhance.gameservice\",\"com.sec.android.app.snsimagecache\",\"com.qualcomm.qti.services.secureui\",\"com.samsung.ucs.agent.ese\",\"com.samsung.android.mhdrservice\",\"com.samsung.dcmservice\",\"com.sec.enterprise.knox.cloudmdm.smdms\",\"com.americanexpress.plenti\",\"com.sec.svoice.lang.en_US\",\"com.sec.svoice.lang.es_US\",\"com.lookout\",\"com.sec.android.app.camera.plb\",\"com.samsung.android.sm.provider\",\"com.sec.epdg\",\"com.sec.android.app.personalization\",\"com.monotype.android.font.cooljazz\",\"com.samsung.android.sdk.handwriting\",\"com.facebook.appmanager\",\"com.samsung.voiceserviceplatform\",\"com.samsung.aasaservice\",\"com.qualcomm.qti.auth.fidosuiservice\",\"com.samsung.android.allshare.service.mediashare\",\"com.sec.android.provider.emergencymode\",\"com.sec.android.app.applinker\",\"com.samsung.android.fingerprint.service\",\"com.sec.knox.switcher\",\"com.sec.android.app.camera\",\"com.samsung.android.contacts\",\"com.samsung.android.app.appupdater\",\"com.samsung.ipservice\",\"com.sec.android.app.magnifier\",\"com.samsung.sec.android.application.csc\",\"com.samsung.upsmtheme\",\"com.samsung.android.app.motionpanoramaviewer\",\"com.samsung.android.video\",\"com.sec.enterprise.knox.shareddevice.keyguard\",\"com.amazon.kindle\"],\"port_path\":\"1/12\",\"processes\":562,\"state\":\"too_hot\",\"temp\":{\"ac\":35.5,\"battery\":30.399999999999999,\"emmc_therm\":31.0,\"max77854-fuelgauge\":30.199999999999999,\"msm_therm\":46.0,\"pa_therm0\":41.0,\"pa_therm1\":37.0,\"pm8004_tz\":37.0,\"pm8994_tz\":42.325000000000003,\"tsens_tz_sensor0\":435.0,\"tsens_tz_sensor1\":448.0,\"tsens_tz_sensor10\":534.0,\"tsens_tz_sensor11\":509.0,\"tsens_tz_sensor12\":493.0,\"tsens_tz_sensor13\":462.0,\"tsens_tz_sensor14\":462.0,\"tsens_tz_sensor15\":462.0,\"tsens_tz_sensor16\":469.0,\"tsens_tz_sensor17\":469.0,\"tsens_tz_sensor18\":456.0,\"tsens_tz_sensor19\":456.0,\"tsens_tz_sensor2\":438.0,\"tsens_tz_sensor20\":475.0,\"tsens_tz_sensor3\":445.0,\"tsens_tz_sensor4\":445.0,\"tsens_tz_sensor5\":467.0,\"tsens_tz_sensor6\":474.0,\"tsens_tz_sensor7\":506.0,\"tsens_tz_sensor8\":547.0,\"tsens_tz_sensor9\":557.0,\"xo_therm_buf\":42.0},\"uptime\":64.519999999999996}},\"disks\":{\"/b\":{\"free_mb\":4298.0,\"size_mb\":26746.5},\"/boot\":{\"free_mb\":40.399999999999999,\"size_mb\":59.899999999999999},\"/home/chrome-bot\":{\"free_mb\":987.20000000000005,\"size_mb\":988.89999999999998},\"/tmp\":{\"free_mb\":974.60000000000002,\"size_mb\":975.89999999999998},\"/var\":{\"free_mb\":769.79999999999995,\"size_mb\":975.89999999999998}},\"gpu\":[\"none\"],\"hostname\":\"skia-rpi-035\",\"ip\":\"192.168.1.135\",\"locale\":\"Unknown\",\"machine_type\":[\"n1-highcpu-4\"],\"nb_files_in_temp\":7,\"periodic_reboot_secs\":43200,\"pid\":572,\"quarantined\":\"No available devices.\",\"ram\":926,\"running_time\":10586,\"sleep_streak\":2,\"started_ts\":1495099895,\"temp\":{\"thermal_zone0\":44.006999999999998},\"uptime\":10624,\"user\":\"chrome-bot\"}", "version": "7139d890ad38500480cd70d61a238f829bedcb01a08f0d2909d048dd23f2098a", "first_seen_ts": "2016-09-09T20:16:24.496470", "last_seen_ts": "2017-05-18T12:28:19.748440", "bot_id": "skia-rpi-035"}`
