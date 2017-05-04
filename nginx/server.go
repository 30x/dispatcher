/*
Copyright © 2016 Apigee Corporation

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package nginx

import (
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"

	"github.com/30x/dispatcher/router"
)

// NginxConfPath for nginx configuration location
const NginxConfPath = "/etc/nginx/nginx.conf"

func shellOut(cmd string, exitOnFailure bool) {
	out, err := exec.Command("sh", "-c", cmd).CombinedOutput()

	if err != nil {
		msg := fmt.Sprintf("Failed to execute (%v): %v, err: %v", cmd, string(out), err)

		if exitOnFailure {
			log.Fatal(msg)
		} else {
			log.Println(msg)
		}
	}
}

func writeNginxConf(conf string) {
	// Create the nginx.conf file based on the template
	if w, err := os.Create(NginxConfPath); err != nil {
		log.Fatalf("Failed to open %s: %v", NginxConfPath, err)
	} else if _, err := io.WriteString(w, conf); err != nil {
		log.Fatalf("Failed to write template %v", err)
	}

	log.Printf("Wrote nginx configuration to %s\n", NginxConfPath)
}

/*
RestartServer restarts nginx using the provided configuration.
*/
func RestartServer(config *router.Config, conf string, exitOnFailure bool) {
	log.Println("Reloading nginx with the following configuration:")

	log.Println(conf)

	// not in mock mode write conf and restart
	if !config.Nginx.RunInMockMode {
		writeNginxConf(conf)
		log.Println("Restarting nginx")
		shellOut("nginx -s reload", exitOnFailure)
	}
}

/*
StartServer starts nginx using the provided configuration.
*/
func StartServer(config *router.Config, conf string) {
	log.Println("Starting nginx with the following configuration:")
	log.Println(conf)

	// not in mock mode write conf and restart
	if !config.Nginx.RunInMockMode {
		writeNginxConf(conf)
		log.Println("Starting nginx")
		shellOut("nginx", true)
	}
}
