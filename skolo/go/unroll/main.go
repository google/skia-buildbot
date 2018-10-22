package main

import (
	"fmt"
	"strings"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/service_accounts"
)

var (
	serviceAccountIpMapping = map[*service_accounts.ServiceAccount][]string{
		service_accounts.ChromeSwarming: []string{"*"},
		service_accounts.ChromiumSwarm:  []string{"*"},
		service_accounts.Jumphost:       []string{"self"},
		service_accounts.RpiMaster:      []string{"192.168.1.98", "192.168.1.99"},
	}
)

func main() {
	common.Init()

	allHostNames := []string{
		"jumphost-internal-01",
		"jumphost-linux-01",
		"jumphost-rpi-01",
		"jumphost-win-02",
		"jumphost-win-03",
	}

	for _, hostname := range allHostNames {
		fmt.Printf("// hostname: %s\n", hostname)
		fmt.Printf("[\n")
		serviceAccounts, ok := service_accounts.JumphostServiceAccountMapping[hostname]
		if !ok {
			sklog.Fatalf("Hostname not in jumphost mapping: %s", hostname)
		}

		// tokens := make(map[string]*metadata.ServiceAccountToken, len(serviceAccounts))
		tokenIpMapping := make(map[string]string, len(serviceAccounts))
		svcAccounts := []string{}
		for _, acct := range serviceAccounts {
			fmt.Printf("  // Clients: %s \n", acct.Nickname)
			fmt.Printf("  {\n")
			ipAddrs, ok := serviceAccountIpMapping[acct]
			if !ok {
				sklog.Fatalf("Service account has no IP address mapping: %s", acct.Email)
			}
			tokenFile := fmt.Sprintf("/var/local/token_%s.json", acct.Nickname)
			keyFile := fmt.Sprintf("/etc/service-accounts/service_account_%s.json", acct.Nickname)
			svcAccounts = append(svcAccounts, acct.Nickname)
			fmt.Printf("    \"project\": %s,\n", acct.Project)
			fmt.Printf("    \"email\": %s,\n", acct.Email)
			fmt.Printf("    \"keyFile\": %s,\n", keyFile)
			fmt.Printf("    \"tokenFile\": %s,\n", tokenFile)
			fmt.Printf("    \"clients\": [\"%s\"],\n", strings.Join(ipAddrs, "\", \""))

			for _, ipAddr := range ipAddrs {
				tokenIpMapping[ipAddr] = tokenFile + "|" + keyFile
				// if _, ok := tokens[tokenFile]; !ok {
				// 	tok, err := metadata.NewServiceAccountToken(tokenFile)
				// 	if err != nil {
				// 		sklog.Fatal(err)
				// 	}
				// 	go tok.UpdateLoop(context.Background())
				// 	tokens[tokenFile] = tok
				// }
			}
			fmt.Printf("  },\n")
		}
		fmt.Printf("]\n\n\n")

		// tokenMapping := make(map[string]*metadata.ServiceAccountToken, len(tokenIpMapping))
		// for ipAddr, tokenFile := range tokenIpMapping {
		// 	tokenMapping[ipAddr] = tokens[tokenFile]
		// }

		// fmt.Printf("%s   %v\n", hostname, svcAccounts)
		// for ipAddr, tokenFile := range tokenIpMapping {
		// 	parts := strings.Split(tokenFile, "|")
		// }
	}
}
