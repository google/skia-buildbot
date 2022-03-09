// This program generates passwordless Remmina[1] RDP/VNC connection profiles to connect to Skolo
// Windows machines from a gLinux workstation (physical or cloudtop). It asks the user to type the
// chrome-bot password and passes it to Remmina via a stdin pipe. Remmina securely stores this
// password in the gnome-keyring or kwallet.
//
// Important: The connection profiles establish an SSH tunnel through the corresponding jumphost
// using passwordless public key authentication. Make sure you're able to passwordlessly SSH into
// rack1, rack2, etc. before using the generated connection profiles.
//
// Usage: Invoke without arguments to generate profiles for all known Skolo Windows machines (e.g.
// "bazel run //skolo/go/generate_remmina_profiles"), then launch Remmina and double-click on a
// connection profile to initiate a VNC or RDP session. Invoke with --help to see additional
// options such as overriding the RDP or VNC port number (e.g.
// "bazel run //skolo/go/generate_remmina_profiles -- --help").
//
// Note that there is nothing specific to Windows about this program, so in theory we could extend
// it to generate connection profiles for Macs or any other OS that can run a VNC or RDP server.
//
// [1] https://remmina.org

package main

import (
	"flag"
	"fmt"
	"html/template"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
)

// machinesByJumphost should be kept in sync with our hosts.yml Ansible file.
var machinesByJumphost = map[string][]string{
	"rack1": {
		"skia-e-win-101",
		"skia-e-win-150",
	},
	"rack2": {
		"skia-e-win-202",
		"skia-e-win-201",
		"skia-e-win-202",
		"skia-e-win-203",
		"skia-e-win-204",
		"skia-e-win-205",
		"skia-e-win-206",
		"skia-e-win-210",
		"skia-e-win-211",
		"skia-e-win-212",
		"skia-e-win-240",
		"skia-e-win-241",
		"skia-e-win-242",
		"skia-e-win-243",
		"skia-e-win-244",
		"skia-e-win-245",
		"skia-e-win-246",
		"skia-e-win-247",
		"skia-e-win-248",
		"skia-e-win-249",
		"skia-e-win-250",
		"skia-e-win-251",
		"skia-e-win-252",
		"skia-e-win-253",
		"skia-e-win-255",
		"skia-e-win-270",
		"skia-e-win-271",
		"skia-e-win-272",
		"skia-e-win-273",
		"skia-e-win-274",
		"skia-e-win-275",
		"skia-e-win-276",
		"skia-e-win-277",
		"skia-e-win-278",
		"skia-e-win-279",
		"skia-e-win-280",
	},
	"rack3": {
		"skia-e-win-301",
		"skia-e-win-302",
		"skia-e-win-304",
		"skia-e-win-305",
		"skia-e-win-306",
		"skia-e-win-310",
		"skia-e-win-311",
		"skia-e-win-312",
		"skia-e-win-341",
		"skia-e-win-342",
		"skia-e-win-343",
		"skia-e-win-344",
		"skia-e-win-345",
		"skia-e-win-346",
		"skia-e-win-347",
		"skia-e-win-348",
		"skia-e-win-349",
		"skia-e-win-353",
		"skia-e-win-354",
		"skia-e-win-355",
	},
}

type Protocol string

const (
	RDP = Protocol("RDP")
	VNC = Protocol("VNC")
)

// connProfile is used to populate a connection profile template.
type connProfile struct {
	Name            string // e.g. "skia-e-win-150 (VNC)"
	Server          string // e.g. "skia-e-win-150:5900"
	SSHTunnelServer string // e.g. "rack1"
}

const rdpConnProfileTmpl = `[remmina]
password=.
gateway_username=
notes_text=
vc=
window_height=768
preferipv6=0
ssh_tunnel_loopback=0
serialname=
websockets=0
printer_overrides=
name={{.Name}}
colordepth=99
security=
precommand=
disable_fastpath=0
left-handed=0
postcommand=
multitransport=0
group=
server={{.Server}}
ssh_tunnel_certfile=
glyph-cache=0
ssh_tunnel_enabled=1
disableclipboard=0
parallelpath=
audio-output=
monitorids=
cert_ignore=1
serialpermissive=0
gateway_server=
protocol=RDP
ssh_tunnel_password=
old-license=0
resolution_mode=2
pth=
loadbalanceinfo=
disableautoreconnect=0
clientbuild=
clientname=
resolution_width=0
drive=
relax-order-checks=0
username=chrome-bot
base-cred-for-gw=0
gateway_domain=
network=none
rdp2tcp=
gateway_password=
rdp_reconnect_attempts=
domain=
serialdriver=
restricted-admin=0
multimon=0
serialpath=
exec=
smartcardname=
enable-autostart=0
usb=
shareprinter=0
ssh_tunnel_passphrase=
shareparallel=0
disablepasswordstoring=0
quality=0
span=0
viewmode=1
parallelname=
ssh_tunnel_auth=3
keymap=
ssh_tunnel_username=chrome-bot
execpath=
shareserial=0
resolution_height=0
timeout=
useproxyenv=0
sharesmartcard=0
freerdp_log_filters=
microphone=
dvc=
ssh_tunnel_privatekey=
gwtransp=http
ssh_tunnel_server={{.SSHTunnelServer}}
ignore-tls-errors=1
window_maximize=0
disable-smooth-scrolling=0
gateway_usage=0
window_width=1024
freerdp_log_level=INFO
sound=off
`

const vncConnProfileTmpl = `[remmina]
password=.
gateway_username=
notes_text=
vc=
window_height=768
preferipv6=0
ssh_tunnel_loopback=0
serialname=
sound=off
disableserverbell=0
printer_overrides=
name={{.Name}}
console=0
colordepth=32
security=
precommand=
disable_fastpath=0
tightencoding=0
left-handed=0
postcommand=
multitransport=0
group=
server={{.Server}}
viewonly=0
ssh_tunnel_certfile=
glyph-cache=0
ssh_tunnel_enabled=1
disableclipboard=0
parallelpath=
audio-output=
monitorids=
cert_ignore=0
serialpermissive=0
gateway_server=
protocol=VNC
ssh_tunnel_password=
old-license=0
resolution_mode=2
pth=
loadbalanceinfo=
disableautoreconnect=0
clientbuild=
clientname=
disablesmoothscrolling=0
resolution_width=0
drive=
relax-order-checks=0
username=
base-cred-for-gw=0
gateway_domain=
network=none
rdp2tcp=
gateway_password=
rdp_reconnect_attempts=
domain=
serialdriver=
restricted-admin=0
multimon=0
serialpath=
exec=
smartcardname=
disableserverinput=0
enable-autostart=0
usb=
shareprinter=0
ssh_tunnel_passphrase=
shareparallel=0
proxy=
disablepasswordstoring=0
quality=0
span=0
viewmode=1
parallelname=
ssh_tunnel_auth=3
keymap=
ssh_tunnel_username=chrome-bot
execpath=
shareserial=0
resolution_height=0
timeout=
useproxyenv=0
sharesmartcard=0
freerdp_log_filters=
microphone=
dvc=
ssh_tunnel_privatekey=
gwtransp=http
ssh_tunnel_server={{.SSHTunnelServer}}
ignore-tls-errors=1
window_maximize=0
showcursor=0
disable-smooth-scrolling=0
gateway_usage=0
disableencryption=0
window_width=1024
encodings=
websockets=0
freerdp_log_level=INFO
`

func main() {
	outputDir := flag.String("output-dir", filepath.Join(os.Getenv("HOME"), ".local", "share", "remmina"), "Output directory.")
	wipeOutputDir := flag.Bool("wipe", false, "Wipe contents of output directory before generating connection profiles.")
	genRDPProfiles := flag.Bool("rdp", true, "Generate RDP connection profiles.")
	genVNCProfiles := flag.Bool("vnc", true, "Generate VNC connection profiles.")
	rdpPort := flag.Int("rdp-port", 3389, "TCP port used for RDP connections.")
	vncPort := flag.Int("vnc-port", 5900, "TCP port used for VNC connections.")
	flag.Parse()

	if *rdpPort < 1 || *rdpPort > 65535 {
		die("Invalid RDP port.")
	}

	if *vncPort < 1 || *vncPort > 65535 {
		die("Invalid VNC port.")
	}

	// Read in the chrome-bot password. Remmina will encrypt it with gnome-keyring or kwallet.
	fmt.Printf("Please enter the chrome-bot password: ")
	chromeBotPassword, err := term.ReadPassword(int(syscall.Stdin))
	ifErrThenDie(err)
	fmt.Println() // term.ReadPassword does not produce a newline when the user presses enter.

	// Create the output directory if it does not exist.
	if _, err := os.Stat(*outputDir); err != nil {
		if os.IsNotExist(err) {
			ifErrThenDie(os.Mkdir(*outputDir, 0755))
		} else {
			die(err)
		}
	}

	// Optionally wipe the contents of the output directory.
	if *wipeOutputDir {
		files, err := ioutil.ReadDir(*outputDir)
		ifErrThenDie(err)
		for _, file := range files {
			ifErrThenDie(os.RemoveAll(filepath.Join(*outputDir, file.Name())))
		}
	}

	// Prepare the requested (protocol, port) pairs.
	type protocolPortPair struct {
		name            Protocol
		port            int
		connProfileTmpl string
	}
	var protocols []protocolPortPair
	if *genRDPProfiles {
		protocols = append(protocols, protocolPortPair{RDP, *rdpPort, rdpConnProfileTmpl})
	}
	if *genVNCProfiles {
		protocols = append(protocols, protocolPortPair{VNC, *vncPort, vncConnProfileTmpl})
	}

	// Generate connection profiles for all known machines.
	for jumphost, machines := range machinesByJumphost {
		for _, machine := range machines {
			for _, protocol := range protocols {
				// Generate Remmina profile.
				profilePath := filepath.Join(*outputDir, fmt.Sprintf("%s-%d-%s.remmina", machine, protocol.port, protocol.name))
				profile := connProfile{
					Name:            fmt.Sprintf("%s:%d (%s)", machine, protocol.port, protocol.name),
					Server:          fmt.Sprintf("%s:%d", machine, protocol.port),
					SSHTunnelServer: jumphost,
				}
				tmpl, err := template.New("connProfileTmpl").Parse(protocol.connProfileTmpl)
				ifErrThenDie(err)
				f, err := os.Create(profilePath)
				ifErrThenDie(err)
				ifErrThenDie(tmpl.Execute(f, profile))
				ifErrThenDie(f.Close())
				fmt.Printf("Generated: %s\n", profilePath)

				// Safely set and store the chrome-bot password. Remmina will read the plain-text
				// password from stdin, and will encrypt it with gnome-keyring or kwallet.
				cmd := exec.Command("remmina", "--update-profile", profilePath, "--set-option", "password")
				stdin, err := cmd.StdinPipe()
				ifErrThenDie(err)
				go func() {
					if _, err := io.WriteString(stdin, string(chromeBotPassword)); err != nil {
						_ = stdin.Close()
						die(err)
					}
					if err := stdin.Close(); err != nil {
						die(err)
					}
				}()
				fmt.Printf("Executing: %s\n", strings.Join(append([]string{cmd.Path}, cmd.Args...), " "))
				out, err := cmd.CombinedOutput()
				if err != nil {
					if execerr, ok := err.(*exec.ExitError); ok {
						fmt.Fprintf(os.Stderr, "Remmina returned a non-zero exit code: %d. Output:\n%s\n", execerr.ExitCode(), string(out))
						os.Exit(1)
					}
					die(err)
				}
			}
		}
	}
}

func ifErrThenDie(err error) {
	if err != nil {
		die(err)
	}
}

func die(msg interface{}) {
	fmt.Fprintf(os.Stderr, "%v\n", msg)
	os.Exit(1)
}
