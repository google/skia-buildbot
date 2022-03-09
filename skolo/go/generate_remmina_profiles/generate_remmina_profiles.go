// This program generates passwordless Remmina[1] RDP/VNC connection profiles to connect to Skolo
// Windows machines from a gLinux workstation (physical or cloudtop). It asks the user to type the
// chrome-bot password and passes it to Remmina via a stdin pipe. Remmina securely stores this
// password in the gnome-keyring or kwallet.
//
// Important: The connection profiles establish an SSH tunnel through the corresponding jumphost
// using passwordless public key authentication. Make sure you're able to passwordlessly SSH into
// rack1, rack2, etc. before using the generated connection profiles.
//
// Usage (from the repository root):
//
//     $ bazel run //skolo/go/generate_remmina_profiles -- skolo/ansible/hosts.yml
//
// Then, launch Remmina and double-click on a connection profile to initiate a session.
//
// Invoke with --help to see additional options such as overriding RDP/VNC port numbers, wiping
// the existing contents of the directory where Remmina profiles live (~/.local/share/remmina),
// customizing said directory, etc.
//
// Note that there is nothing specific to Windows about this program, so in theory we could extend
// it to generate connection profiles for Macs or any other OSes that can run a VNC or RDP server.
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
	"regexp"
	"strings"
	"syscall"

	"golang.org/x/term"
)

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

var (
	rackRegexp       = regexp.MustCompile(`^rack\d+$`)
	winMachineRegexp = regexp.MustCompile(`^skia-e-win-\d+$`)
)

func main() {
	outputDir := flag.String("output-dir", filepath.Join(os.Getenv("HOME"), ".local", "share", "remmina"), "Output directory.")
	wipeOutputDir := flag.Bool("wipe", false, "Wipe contents of output directory before generating connection profiles.")
	genRDPProfiles := flag.Bool("rdp", true, "Generate RDP connection profiles.")
	genVNCProfiles := flag.Bool("vnc", true, "Generate VNC connection profiles.")
	rdpPort := flag.Int("rdp-port", 3389, "TCP port used for RDP connections.")
	vncPort := flag.Int("vnc-port", 5900, "TCP port used for VNC connections.")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: %s [OPTIONS] <path to Ansible inventory file>\n", os.Args[0])
		flag.PrintDefaults()
	}

	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(1)
	}

	ansibleInventoryFilePath := flag.Arg(0)

	// If running via "bazel run", change into the directory where Bazel was invoked. This is
	// necessary to correctly compute the absolute path of the Ansible inventory file.
	if os.Getenv("BUILD_WORKING_DIRECTORY") != "" {
		ifErrThenDie(os.Chdir(os.Getenv("BUILD_WORKING_DIRECTORY")))
	}

	if *rdpPort < 1 || *rdpPort > 65535 {
		die("Invalid RDP port.")
	}

	if *vncPort < 1 || *vncPort > 65535 {
		die("Invalid VNC port.")
	}

	machinesByJumphost, err := getMachinesByJumphost(ansibleInventoryFilePath)
	ifErrThenDie(err)

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

func getMachinesByJumphost(ansibleInventoryFilePath string) (map[string][]string, error) {
	racks, err := queryAnsibleInventory(ansibleInventoryFilePath, "all", rackRegexp)
	if err != nil {
		return nil, err
	}
	machinesByJumphost := map[string][]string{}
	for _, rack := range racks {
		hosts, err := queryAnsibleInventory(ansibleInventoryFilePath, rack+"_win", winMachineRegexp)
		ifErrThenDie(err)
		machinesByJumphost[rack] = hosts
	}
	return machinesByJumphost, nil
}

// queryAnsibleInventory returns a list hosts that match the given Ansible pattern (e.g. "all",
// "rack1", etc.) and the given regular expression.
func queryAnsibleInventory(ansibleInventoryFilePath, pattern string, hostsFilter *regexp.Regexp) ([]string, error) {
	cmd := exec.Command("ansible", "--inventory", ansibleInventoryFilePath, "--list-hosts", pattern)
	outBytes, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	rawLines := strings.Split(string(outBytes), "\n")

	// The output looks like so:
	//
	//     $ ansible --inventory skolo/ansible/hosts.yml --list-hosts all
	//       hosts (275):
	//         rack1
	//         rack2
	//         rack3
	//         ...
	//
	// Thus, we discard the first line and trim any leading/trailing spaces on the remaining lines.
	rawLines = rawLines[1:]
	var hosts []string
	for _, line := range rawLines {
		line = strings.TrimSpace(line)
		if hostsFilter.MatchString(line) {
			hosts = append(hosts, line)
		}
	}

	return hosts, nil
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
