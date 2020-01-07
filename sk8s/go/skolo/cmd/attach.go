package cmd

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os/user"
	"path/filepath"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/ssh"
	"gopkg.in/yaml.v2"
)

type clusterYaml struct {
	Cluster map[string]string `yaml:"cluster"`
	Name    string            `yaml:"name"`
}

type clusterConfigYaml struct {
	Clusters       []clusterYaml `yaml:"clusters"`
	Contexts       interface{}   `yaml:"contexts"`
	CurrentContext interface{}   `yaml:"current-context"`
	Kind           interface{}   `yaml:"kind"`
	Preferences    interface{}   `yaml:"preferences"`
	Users          interface{}   `yaml:"users"`
}

// attachCmd represents the attach command
var attachCmd = &cobra.Command{
	Use:   "attach",
	Short: "Attach to all the kubernetes cluster api endpoints to this computer.",
	Long: `Attaches all the kubernetes cluster api endpoints in /kube/clusters/config.json
that have type 'k3s'. They will remain attached, allowing pushk to push to all clusters.

`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Println("attach called")

		clusters := clusterConfig.GetStringMap("clusters")
		for name, _ := range clusters {
			cfg := clusterConfig.GetStringMapString("clusters." + name)
			fmt.Printf("%s - %s %s %s\n", name, cfg["type"], cfg["ip"], cfg["port"])
		}

		// Load the users private ssh key.
		u, err := user.Current()
		if err != nil {
			return fmt.Errorf("Failed to get current user: %s", err)
		}

		privateBytes, err := ioutil.ReadFile(filepath.Join(u.HomeDir, ".ssh", "id_rsa"))
		if err != nil {
			return fmt.Errorf("Failed to load private key: %s", err)
		}

		private, err := ssh.ParsePrivateKey(privateBytes)
		if err != nil {
			return fmt.Errorf("Failed to parse private key: %s", err)
		}

		// Connect to the jumphost.
		config := &ssh.ClientConfig{
			User: "chrome-bot",
			Auth: []ssh.AuthMethod{
				ssh.PublicKeys(private),
			},
			HostKeyCallback: ssh.InsecureIgnoreHostKey(),
		}
		client, err := ssh.Dial("tcp", "100.115.95.135:22", config)
		if err != nil {
			return fmt.Errorf("Failed to dial: %s", err)
		}

		session, err := client.NewSession()
		if err != nil {
			return fmt.Errorf("Failed to create session: %s", err)
		}
		defer session.Close()

		// Once a Session is created, you can execute a single command on
		// the remote side using the Run method.
		var b bytes.Buffer
		session.Stdout = &b
		if err := session.Run("sudo kubectl config view --raw"); err != nil {
			return fmt.Errorf("Failed to run: %s", err.Error())
		}
		fmt.Println(b.String())

		// DIR=${HOME}/.config/skia-infra/skolo/skolo-${RACK}

		var parsedConfig clusterConfigYaml
		if err := yaml.Unmarshal(b.Bytes(), &parsedConfig); err != nil {
			return fmt.Errorf("Failed to parse kubernetes cluster config yaml: %s", err)
		}
		fmt.Printf("%s", parsedConfig.Clusters[0].Cluster["server"])

		return nil
	},
}

func init() {
	rootCmd.AddCommand(attachCmd)

	// Here you will define your flags and configuration settings.

	// Cobra supports Persistent Flags which will work for this command
	// and all subcommands, e.g.:
	// attachCmd.PersistentFlags().String("foo", "", "A help for foo")

	// Cobra supports local flags which will only run when this command
	// is called directly, e.g.:
	// attachCmd.Flags().BoolP("toggle", "t", false, "Help message for toggle")
}
