package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/crypto/ssh"

	"github.com/kevinburke/ssh_config"
	"github.com/kr/pty"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	script      = flag.String("script", "", "Path to a Python script to run.")
	includeBots = common.NewMultiStringFlag("include_bot", nil, "If specified, treated as a white list of bots which will be affected, calculated AFTER the dimensions is computed. Can be simple strings or regexes")
)

func main() {
	// Setup, parse args.
	defer common.LogPanic()
	common.Init()

	// ctx := context.Background()
	if *script == "" {
		sklog.Fatal("--script is required.")
	}

	includeRegs, err := parseRegex(*includeBots)
	if err != nil {
		sklog.Fatal(err)
	}

	// Obtain the list of VMs that should be used.
	// tokenSrc, err := google.DefaultTokenSource(ctx)
	// if err != nil {
	// 	sklog.Fatal(err)
	// }

	f, err := os.Open(filepath.Join(os.Getenv("HOME"), ".ssh", "config"))
	if err != nil {
		sklog.Fatal(err)
	}
	defer util.Close(f)

	cfg, err := ssh_config.Decode(f)
	if err != nil {
		sklog.Fatal(err)
	}

	for _, host := range cfg.Hosts {
		hostAlias := host.Patterns[0].String()
		hostConf, ok := filteredMap(host)
		if ok {
			ipAddress := hostConf["HostName"]
			idFile := hostConf["IdentityFile"]

			var method ssh.AuthMethod = nil

		}
	}

	bots := []string{"skia-gce-021"}

	// Trigger the task on each bot.
	for _, bot := range bots {
		if !matchesAny(bot, includeRegs) {
			sklog.Debugf("Skipping %s because it isn't in the whitelist", bot)
			continue
		}

		//		baseCmd := "sudo DEBIAN_FRONTEND=noninteractive apt -o quiet=2 --assume-yes -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold "
		lines, err := execCmds(ipAddress, method, []string{
			"ls -lah /",
		})
		if err != nil {
			sklog.Fatal(err)
		}
		fmt.Printf("\n\n%s\n\n", strings.Join(lines, "\n"))
	}
}

func filteredMap(host *ssh_config.Host) (map[string]string, bool) {
	ret := map[string]string{}
	for _, node := range host.Nodes {
		kv, ok := node.(*ssh_config.KV)
		if ok {
			ret[kv.Key] = kv.Value
		}
	}
	_, ok := ret["HostName"]
	return ret, ok
}

func runCommands(botName string, cmds []string) error {
	subProcess := exec.Command("gcloud", "compute", "ssh", botName)

	f, err := pty.Start(subProcess)
	if err != nil {
		sklog.FmtErrorf("Error starting command: %s", err)
	}
	defer util.Close(f)

	go func() {
		for _, cmd := range cmds {
			_, err := f.WriteString(cmd + "\n")
			if err != nil {
				sklog.Errorf("Error writing cmd '%s': %s", cmd, err)
			}
		}
		_, err = f.WriteString("exit")
		if err != nil {
			sklog.Errorf("Error writing to 'exit' to stdin: %s", err)
		}
	}()
	io.Copy(os.Stdout, f)

	// stdin, err := subProcess.StdinPipe()
	// if err != nil {
	// 	return sklog.FmtErrorf("Error getting stdin: %s", err)
	// }
	// defer stdin.Close() // the doc says subProcess.Wait will close it, but I'm not sure, so I kept this line

	// subProcess.Stdout = os.Stdout
	// subProcess.Stderr = os.Stderr

	// fmt.Println("START")
	// if err = subProcess.Start(); err != nil {
	// 	return sklog.FmtErrorf("Error starting shell: %s", err)
	// }

	// for _, cmd := range cmds {
	// 	_, err := io.WriteString(stdin, cmd+"\n")
	// 	if err != nil {
	// 		return sklog.FmtErrorf("Error writing to stdin: %s", err)
	// 	}
	// }
	// _, err = io.WriteString(stdin, "exit")
	// if err != nil {
	// 	return sklog.FmtErrorf("Error writing to 'exit' to stdin: %s", err)
	// }
	// subProcess.Wait()
	return nil
}

func parseRegex(flags common.MultiString) (retval []*regexp.Regexp, e error) {
	if len(flags) == 0 {
		return retval, nil
	}

	for _, s := range flags {
		r, err := regexp.Compile(s)
		if err != nil {
			return nil, err
		}
		retval = append(retval, r)
	}
	return retval, nil
}

func matchesAny(s string, xr []*regexp.Regexp) bool {
	if len(xr) == 0 {
		return true
	}
	for _, r := range xr {
		if r.MatchString(s) {
			return true
		}
	}
	return false
}

const DEFAULT_USER = "default"

func newClient(ipAddress string, method ssh.AuthMethod) (*ssh.Client, error) {
	sshConfig := &ssh.ClientConfig{
		User:            DEFAULT_USER,
		Auth:            []ssh.AuthMethod{method},
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	client, err := ssh.Dial("tcp", ipAddress, sshConfig)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// ExecCmds, see CommandRunner
func execCmds(ipAddress string, method ssh.AuthMethod, cmds []string) ([]string, error) {
	// The EdgeSwitch server doesn't like to re-use a client. So we create
	// a new connection for every series of commands.
	client, err := newClient(ipAddress, method)
	if err != nil {
		return nil, err
	}
	defer util.Close(client)

	session, err := client.NewSession()
	if err != nil {
		return nil, err
	}
	defer util.Close(session)

	// Set a terminal with many lines so we are not paginated.
	if err := session.RequestPty("xterm", 80, 5000, nil); err != nil {
		return nil, fmt.Errorf("Error: Could not retrieve pseudo terminal: %s", err)
	}

	stdinPipe, err := session.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdoutPipe, err := session.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := session.Shell(); err != nil {
		return nil, err
	}

	for _, cmd := range cmds {
		sklog.Infof("Executing: %s", cmd)
		if _, err := io.WriteString(stdinPipe, cmd+"\n"); err != nil {
			return nil, err
		}
	}
	if _, err := io.WriteString(stdinPipe, "exit\n"); err != nil {
		return nil, err
	}
	// Get the output and return it.
	var buf bytes.Buffer
	if _, err := io.Copy(&buf, stdoutPipe); err != nil {
		return nil, err
	}

	// Strip out empty lines and all lines with the prompt.
	lines := strings.Split(buf.String(), "\n")
	ret := make([]string, 0, len(lines))
	for _, line := range lines {
		oneLine := strings.TrimSpace(line)
		ret = append(ret, oneLine)
	}

	return ret, nil
}

func PublicKeyFile(file string) ssh.AuthMethod {
	buffer, err := ioutil.ReadFile(file)
	if err != nil {
		return nil
	}

	key, err := ssh.ParsePrivateKey(buffer)
	if err != nil {
		return nil
	}
	return ssh.PublicKeys(key)
}
