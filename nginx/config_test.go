package nginx

import (
	//	"bytes"
	//	"encoding/base64"
	"bytes"
	"github.com/30x/dispatcher/router"
	"log"
	"strings"
	"testing"
	//	"k8s.io/kubernetes/pkg/api"
)

var config *router.Config

func init() {
	resetConf()
}

func resetConf() {
	envConfig, err := router.ConfigFromEnv()

	if err != nil {
		log.Fatalf("Unable to get configuration from environment: %v", err)
	}

	config = envConfig
}

func getConfig() templateDataT {
	resetConf()
	return templateDataT{
		APIKeyHeader: nginxAPIKeyHeader,
		Hosts:        make(map[string]*hostT),
		Upstreams:    make(map[string]*upstreamT),
		Config:       config,
	}
}

func TestPartialDefaultServer(t *testing.T) {
	tmplData := getConfig()
	// Set nginx port to custom value
	tmplData.Config.Nginx.Port = 1234

	var doc bytes.Buffer

	if err := nginxTemplate.ExecuteTemplate(&doc, "default-server", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	// Test for configured port
	if idx := strings.Index(doc.String(), "listen 1234 default_server;"); idx < 0 {
		t.Fatalf("Expected default server to listen on custom port and default_server")
	}

	if idx := strings.Index(doc.String(), "return 444;"); idx < 0 {
		t.Fatalf("Expected default server to only return 444;")
	}

	// TODO: Make test return 444 when it's configurable to a target

}

func TestPartialBaseConfig(t *testing.T) {
	tmplData := getConfig()

	var doc bytes.Buffer

	if err := nginxTemplate.ExecuteTemplate(&doc, "base-config", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	expected := `events {
  worker_connections 1024;
}`

	if doc.String() != expected {
		t.Fatalf("Base config does not match expected")
	}
}

func TestPartialDefaultLocation(t *testing.T) {
	tmplData := getConfig()

	var doc bytes.Buffer

	if err := nginxTemplate.ExecuteTemplate(&doc, "default-location", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	expected := `# Here to avoid returning the nginx welcome page for servers that do not have a "/" location.  (Issue #35)
    location / {
      return 404;
    }`

	if doc.String() != expected {
		t.Fatalf("Default location does not match expected")
	}
}

func TestPartialHttpPreamble(t *testing.T) {

	tmplData := getConfig()

	var doc bytes.Buffer

	if err := nginxTemplate.ExecuteTemplate(&doc, "http-preamble", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	searches := []string{
		"types_hash_max_size 2048;",
		"server_names_hash_max_size 512;",
		"server_names_hash_bucket_size 64;",
		"client_max_body_size 0;", // default max body size 0
		"proxy_http_version 1.1;",
		"proxy_set_header Connection $p_connection;",
		"proxy_set_header Host $http_host;",
		"proxy_set_header Upgrade $http_upgrade;",
		`map $http_connection $p_connection {
    default $http_connection;
    ''      close;
  }`,
	}

	for _, search := range searches {
		if idx := strings.Index(doc.String(), search); idx < 0 {
			t.Fatalf("Did not find %s in http-preamble;", search)
		}
	}

	tmplData.Config.Nginx.MaxClientBodySize = "34mb"

	if err := nginxTemplate.ExecuteTemplate(&doc, "http-preamble", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}
	if idx := strings.Index(doc.String(), "client_max_body_size 34mb;"); idx < 0 {
		t.Fatalf("MaxClientBodySize in config did not change nginx config;")
	}

}

func TestDefaultConfig(t *testing.T) {
	tmplData := getConfig()

	var doc bytes.Buffer

	if err := nginxTemplate.ExecuteTemplate(&doc, "nginx", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	var preamble bytes.Buffer
	if err := nginxTemplate.ExecuteTemplate(&preamble, "http-preamble", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	var defServer bytes.Buffer
	if err := nginxTemplate.ExecuteTemplate(&defServer, "default-server", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	var baseConfig bytes.Buffer
	if err := nginxTemplate.ExecuteTemplate(&baseConfig, "base-config", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	if idx := strings.Index(doc.String(), preamble.String()); idx < 0 {
		t.Fatalf("Default config should contain http-preamble;")
	}

	if idx := strings.Index(doc.String(), defServer.String()); idx < 0 {
		t.Fatalf("Default config should contain default-server;")
	}

	if idx := strings.Index(doc.String(), baseConfig.String()); idx < 0 {
		t.Fatalf("Default config should contain base-config;")
	}

	if strings.Count(doc.String(), "# Upstream for") > 0 {
		t.Fatalf("Default config should not have any upstream servers;")
	}

	if strings.Count(doc.String(), "server {") > 1 {
		t.Fatalf("Default config should not have any servers besides default;")
	}
}

func TestGetConfNoPodsOnlyNamespace(t *testing.T) {
	cache := router.NewCache()

	cache.Namespaces["test-namespace"] = &router.Namespace{
		Name:         "test-namespace",
		Hosts:        map[string]router.HostOptions{"api.ex.net": router.HostOptions{}},
		Organization: "some-org",
		Environment:  "test",
	}

	cache.Namespaces["test2-namespace"] = &router.Namespace{
		Name:         "test2-namespace",
		Hosts:        map[string]router.HostOptions{"api.ag.net": router.HostOptions{}, "api.v2.ag.net": router.HostOptions{}},
		Organization: "some-other-org",
		Environment:  "test",
	}

	doc := GetConf(config, cache)

	if strings.Count(doc, "server {") != 3 {
		t.Fatalf("Expected 3 server { in generated config, 1 default and 2 for each namespace")
	}

	if idx := strings.Index(doc, "server_name api.ex.net;"); idx < 0 {
		t.Fatalf("Expected single server_name for namespace")
	}

	if idx := strings.Index(doc, "server_name api.ag.net api.v2.ag.net;"); idx < 0 {
		t.Fatalf("Expected multiple server_name for namespace")
	}

	tmplData := getConfig()
	var defaultLocation bytes.Buffer
	if err := nginxTemplate.ExecuteTemplate(&defaultLocation, "default-location", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	if strings.Count(doc, defaultLocation.String()) != 2 {
		t.Fatalf("Expected 2 default locations for each namespace")
	}
}

func TestGetConfCheckUpstreams(t *testing.T) {
	cache := router.NewCache()

	cache.Namespaces["test-namespace"] = &router.Namespace{
		Name:         "test-namespace",
		Hosts:        map[string]router.HostOptions{"api.ex.net": router.HostOptions{}},
		Organization: "some-org",
		Environment:  "test",
	}

	cache.Pods["some-pod1"] = &router.PodWithRoutes{
		Name:      "some-pod1",
		Namespace: "test-namespace",
		Routes: []*router.Route{&router.Route{
			Incoming: &router.Incoming{"/users"},
			Outgoing: &router.Outgoing{IP: "1.2.3.4", Port: "8080"},
		}},
	}

	cache.Pods["some-pod2"] = &router.PodWithRoutes{
		Name:      "some-pod2",
		Namespace: "test-namespace",
		Routes: []*router.Route{&router.Route{
			Incoming: &router.Incoming{"/users"},
			Outgoing: &router.Outgoing{IP: "1.5.6.7", Port: "3000"},
		}},
	}

	doc := GetConf(config, cache)

	if strings.Count(doc, "# Upstream for") != 1 {
		t.Fatalf("Expected only 1 upstream")
	}
	if idx := strings.Index(doc, "server 1.2.3.4:8080;"); idx < 0 {
		t.Fatalf("Expected pod1 as a target with target 1.2.3.4:8080")
	}
	if idx := strings.Index(doc, "server 1.5.6.7:3000;"); idx < 0 {
		t.Fatalf("Expected pod1 as a target with target 1.5.6.7:3000")
	}
}

func TestGetConfCheckLocationNoSecret(t *testing.T) {
	cache := router.NewCache()

	cache.Namespaces["test-namespace"] = &router.Namespace{
		Name:         "test-namespace",
		Hosts:        map[string]router.HostOptions{"api.ex.net": router.HostOptions{}},
		Organization: "some-org",
		Environment:  "test",
	}

	cache.Pods["some-pod1"] = &router.PodWithRoutes{
		Name:      "some-pod1",
		Namespace: "test-namespace",
		Routes: []*router.Route{&router.Route{
			Incoming: &router.Incoming{"/users"},
			Outgoing: &router.Outgoing{IP: "1.2.3.4", Port: "8080"},
		}},
	}

	cache.Pods["some-pod2"] = &router.PodWithRoutes{
		Name:      "some-pod2",
		Namespace: "test-namespace",
		Routes: []*router.Route{&router.Route{
			Incoming: &router.Incoming{"/users"},
			Outgoing: &router.Outgoing{IP: "1.5.6.7", Port: "3000"},
		}},
	}

	doc := GetConf(config, cache)

	if strings.Count(doc, "location /users") != 1 {
		t.Fatalf("Expected one location /useres in config")
	}

	if strings.Count(doc, "# Check the Routing API Key") != 0 {
		t.Fatalf("Should not have any routing key checks")
	}

	if idx := strings.Index(doc, "proxy_pass http://upstream"); idx < 0 {
		t.Fatalf("Expected proxy_pass to upstream")
	}
}

func TestGetConfCheckLocationWithSecret(t *testing.T) {
	cache := router.NewCache()

	cache.Namespaces["test-namespace"] = &router.Namespace{
		Name:         "test-namespace",
		Hosts:        map[string]router.HostOptions{"api.ex.net": router.HostOptions{}},
		Organization: "some-org",
		Environment:  "test",
	}

	cache.Secrets["test-namespace"] = &router.Secret{Namespace: "test-namespace", Data: []byte{'A', 'B', 'C'}}

	cache.Pods["some-pod1"] = &router.PodWithRoutes{
		Name:      "some-pod1",
		Namespace: "test-namespace",
		Routes: []*router.Route{&router.Route{
			Incoming: &router.Incoming{"/users"},
			Outgoing: &router.Outgoing{IP: "1.2.3.4", Port: "8080"},
		}},
	}

	cache.Pods["some-pod2"] = &router.PodWithRoutes{
		Name:      "some-pod2",
		Namespace: "test-namespace",
		Routes: []*router.Route{&router.Route{
			Incoming: &router.Incoming{"/users"},
			Outgoing: &router.Outgoing{IP: "1.5.6.7", Port: "3000"},
		}},
	}

	doc := GetConf(config, cache)

	if strings.Count(doc, "location /users") != 1 {
		t.Fatalf("Expected one location /users in config")
	}

	if idx := strings.Index(doc, "proxy_pass http://upstream"); idx < 0 {
		t.Fatalf("Expected proxy_pass to upstream")
	}

	expected := `if ($http_x_routing_api_key != "QUJD") {
        return 403;
      }`

	if idx := strings.Index(doc, expected); idx < 0 {
		t.Fatalf("Expected to have key check")
	}
}

func TestGetConfCheckLocationNoDefaultLocation(t *testing.T) {
	cache := router.NewCache()

	cache.Namespaces["test-namespace"] = &router.Namespace{
		Name:         "test-namespace",
		Hosts:        map[string]router.HostOptions{"api.ex.net": router.HostOptions{}},
		Organization: "some-org",
		Environment:  "test",
	}

	cache.Secrets["test-namespace"] = &router.Secret{Namespace: "test-namespace", Data: []byte{'A', 'B', 'C'}}

	cache.Pods["some-pod1"] = &router.PodWithRoutes{
		Name:      "some-pod1",
		Namespace: "test-namespace",
		Routes: []*router.Route{&router.Route{
			Incoming: &router.Incoming{"/users"},
			Outgoing: &router.Outgoing{IP: "1.2.3.4", Port: "8080"},
		}},
	}

	cache.Pods["some-pod2"] = &router.PodWithRoutes{
		Name:      "some-pod2",
		Namespace: "test-namespace",
		Routes: []*router.Route{&router.Route{
			Incoming: &router.Incoming{"/"},
			Outgoing: &router.Outgoing{IP: "1.5.6.7", Port: "3000"},
		}},
	}

	doc := GetConf(config, cache)

	if strings.Count(doc, "location /users {") != 1 {
		t.Fatalf("Expected location /users in config")
	}

	if strings.Count(doc, "location / {") != 1 {
		t.Fatalf("Expected location / in config")
	}

	tmplData := getConfig()
	var defaultLocation bytes.Buffer
	if err := nginxTemplate.ExecuteTemplate(&defaultLocation, "default-location", tmplData); err != nil {
		t.Fatalf("Failed to write template %v", err)
	}

	if strings.Count(doc, defaultLocation.String()) != 0 {
		t.Fatalf("Config should not have a default location for server")
	}
}
