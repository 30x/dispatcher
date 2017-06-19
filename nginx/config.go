package nginx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/30x/dispatcher/router"
	"hash/fnv"
	"io/ioutil"
	"log"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"text/template"
)

const (
	nginxConfTmpl = `
{{template "base-config" .}}

http {
  {{template "http-preamble" .}}

  {{range $key, $upstream := .Upstreams}}
  # Upstream for {{$upstream.Path}} traffic on namespace {{$upstream.Namespace}}
  upstream {{$upstream.Name}} {
    keepalive 1024;
    {{range $server := $upstream.Servers}}
    # Pod {{$server.Pod.Name}} (namespace: {{$server.Pod.Namespace}})
    server {{$server.Target}}{{if $server.Weight}} weight={{$server.Weight}}{{end}};
    {{if and $.Config.Nginx.EnableHealthChecks $upstream.HealthCheck }}
    {{template "upstream-healthcheck" $upstream.HealthCheck}}
    {{- end}}
    {{- end}}
  }
  {{end -}}

  {{range $host, $server := .Hosts}}
  server {
    listen {{if $server.SSL -}}{{$.Config.Nginx.SSLPort}} ssl{{else}}{{$.Config.Nginx.Port}}{{end}};
    server_name {{$host}};
    {{if $server.SSL -}} {{template "ssl-host" $server.SSL}}{{- end}}
    {{if $server.NeedsDefaultLocation -}} {{template "default-location" $}}{{- end}}

    {{range $path, $location := $server.Locations -}}
    location {{$path}} {
      {{if ne $location.Secret "" -}}
      # Check the Routing API Key (namespace: {{$location.Namespace}})
      if ($http_{{$.APIKeyHeader}} != "{{$location.Secret}}") {
        return 403;
      }
      {{- end}}
      # Force keepalive
      proxy_http_version 1.1;
      proxy_set_header Connection "";

      # Set Host Header, if not nginx will use upsteam's name 
      proxy_set_header Host $http_host;

      #Upstream {{$location.Upstream}}
      proxy_pass http://{{$location.Upstream}}{{if $location.TargetPath}}{{$location.TargetPath}}{{end}};
    }

    {{end}}
  }
  {{end}}

  {{template "default-server" .}}
  {{if .Config.Nginx.SSLEnabled}}{{template "default-ssl-server" .}}{{end}}
}
`

	partialsTmpl = `
{{define "base-config" -}}
events {
  worker_connections  81920;
  multi_accept        on;
}
{{- end}}

{{define "default-server" -}}
  # Default server that will just close the connection as if there was no server available
  server {
    listen {{.Config.Nginx.Port}} default_server;

    location = {{.Config.Nginx.StatusPath}} {
      return 200;
    }

    location / {
      return 444;
    }
  }
{{- end}}

{{define "default-ssl-server" -}}
  # Default server that will just close the connection as if there was no server available
  server {
    listen {{.Config.Nginx.SSLPort}} default_server ssl;
    # SSL Options
    ssl_ciphers HIGH:!aNULL:!MD5:!DH+3DES:!kEDH;
    ssl_protocols TLSv1 TLSv1.1 TLSv1.2;
    ssl_certificate {{.Config.Nginx.SSLCert}};
    ssl_certificate_key {{.Config.Nginx.SSLKey}};

    location = {{.Config.Nginx.StatusPath}} {
      return 200;
    }

    location / {
      return 444;
    }
  }
{{- end}}

{{define "default-location" -}}
    # Here to avoid returning the nginx welcome page for servers that do not have a "/" location.  (Issue #35)
    location / {
      {{.DefaultLocationReturn}}
    }
{{- end}}

{{define "ssl-host" -}}
    # SSL Options
    ssl_ciphers HIGH:!aNULL:!MD5:!DH+3DES:!kEDH;
    ssl_protocols TLSv1 TLSv1.1 TLSv1.2;
    ssl_certificate {{.Certificate}};
    ssl_certificate_key {{.Key}};
    {{if .ClientCertificate -}}
    ssl_client_certificate {{.ClientCertificate}};
    ssl_verify_client on;
    {{- end}}
{{- end}}

{{define "upstream-healthcheck" -}}
    # Upstream Health Check for nginx_upstream_check_module - https://github.com/yaoweibin/nginx_upstream_check_module
    {{- if .HTTPCheck}}
    check interval={{.IntervalMs}} rise={{.HealthyThreshold}} fall={{.UnhealthyThreshold}} timeout={{.TimeoutMs}} port={{.Port}} type=http;
    check_http_send "{{.Method}} {{.Path}} HTTP/1.0\r\n\r\n";
    check_http_expect_alive http_2xx;
    {{- else -}}
    check interval={{.IntervalMs}} rise={{.HealthyThreshold}} fall={{.UnhealthyThreshold}} timeout={{.TimeoutMs}} port={{.Port}} type=tcp;
    {{- end}}
{{- end}}

{{define "http-preamble" -}}
  # http://nginx.org/en/docs/http/ngx_http_core_module.html
  types_hash_max_size 2048;
  server_names_hash_max_size 512;
  server_names_hash_bucket_size 64;

  # Maximum body size in request
  client_max_body_size {{.Config.Nginx.MaxClientBodySize}};

  # Force HTTP 1.1 for upstream requests
  proxy_http_version 1.1;
  
  # timeout after 5s for upstreams
  proxy_connect_timeout 5s;
  
  # Don't proxy req body in nginx, send directly to upstream
  proxy_request_buffering off;

  # When nginx proxies to an upstream, the default value used for 'Connection' is 'close'.  We use this variable to do
  # the same thing so that whenever a 'Connection' header is in the request, the variable reflects the provided value
  # otherwise, it defaults to 'close'.  This is opposed to just using "proxy_set_header Connection $http_connection"
  # which would remove the 'Connection' header from the upstream request whenever the request does not contain a
  # 'Connection' header, which is a deviation from the nginx norm.
  map $http_connection $p_connection {
    default $http_connection;
    ''      close;
  }

  # Pass through the appropriate headers
  proxy_set_header Connection $p_connection;
  proxy_set_header Upgrade $http_upgrade;
{{- end}}
`
)

type sslOptions struct {
	Certificate       string  // path to certficate fil
	Key               string  // path to key
	ClientCertificate *string // optional path to a client verification certificate
}

type hostT struct {
	HostOptions          *router.HostOptions
	Locations            map[string]*locationT
	SSL                  *sslOptions
	NeedsDefaultLocation bool
}

type locationT struct {
	Namespace  string
	Path       string
	Upstream   string
	Secret     string
	TargetPath *string
}

type upstreamT struct {
	Namespace   string
	Name        string
	Path        string
	Servers     serversT
	HealthCheck *router.HealthCheck
}

type serverT struct {
	Pod    *router.PodWithRoutes
	Weight *uint
	Target string
}

type templateDataT struct {
	APIKeyHeader          string
	DefaultLocationReturn string
	Hosts                 map[string]*hostT
	Upstreams             map[string]*upstreamT
	Config                *router.Config
}

type serversT []*serverT

func (slice serversT) Len() int {
	return len(slice)
}

func (slice serversT) Less(i, j int) bool {
	return slice[i].Pod.Name < slice[j].Pod.Name
}

func (slice serversT) Swap(i, j int) {
	slice[i], slice[j] = slice[j], slice[i]
}

var nginxTemplate *template.Template
var nginxAPIKeyHeader string

func init() {

	// Parse the default nginx.conf template
	nginxTemplate = template.Must(template.New("nginx").Parse(nginxConfTmpl))

	_, err := nginxTemplate.Parse(partialsTmpl)
	if err != nil {
		log.Fatal("Failed parsing template", err)
	}
}

func defaultReturnFromConfig(config *router.Config) string {
	code, err := strconv.Atoi(config.Nginx.DefaultLocationReturn)
	if err == nil {
		// use as return code
		return fmt.Sprintf("return %d;", code)
	}

	// string use as upstream
	return fmt.Sprintf("proxy_pass %s;", config.Nginx.DefaultLocationReturn)
}

func processSSLOptions(ns, hostname string, opts *router.SSLOptions, cache *router.Cache, config *router.Config) (*sslOptions, error) {
	nsSecret, ok := cache.Secrets[ns]
	if !ok {
		return nil, fmt.Errorf("namespace secret missing for %s", ns)
	}

	if _, ok := nsSecret.Fields[opts.Certificate.ValueFrom.SecretKeyRef.Key]; !ok {
		return nil, fmt.Errorf("namespace secret missing certificate field %s", opts.Certificate.ValueFrom.SecretKeyRef.Key)
	}

	if _, ok := nsSecret.Fields[opts.Key.ValueFrom.SecretKeyRef.Key]; !ok {
		return nil, fmt.Errorf("namespace secret missing key field %s", opts.Key.ValueFrom.SecretKeyRef.Key)
	}

	baseDir := fmt.Sprintf("%s/%s", config.Nginx.SSLCertificateDir, hostname)
	ssl := &sslOptions{
		Certificate: fmt.Sprintf("%s/certificate.crt", baseDir),
		Key:         fmt.Sprintf("%s/certificate.key", baseDir),
	}

	if opts.ClientCertficate != nil {
		if _, ok := nsSecret.Fields[opts.ClientCertficate.ValueFrom.SecretKeyRef.Key]; !ok {
			return nil, fmt.Errorf("namespace secret missing certificate field %s", opts.ClientCertficate.ValueFrom.SecretKeyRef.Key)
		}

		clientCertificateFile := fmt.Sprintf("%s/clientCertificate.crt", baseDir)
		ssl.ClientCertificate = &clientCertificateFile
	}

	// If not running in mock mode write cert to disk
	if !config.Nginx.RunInMockMode {
		err := os.MkdirAll(baseDir, 0700)
		if err != nil {
			return nil, err
		}

		err = ioutil.WriteFile(ssl.Certificate, nsSecret.Fields[opts.Certificate.ValueFrom.SecretKeyRef.Key], 0644)
		if err != nil {
			return nil, fmt.Errorf("failed to write certificate to %s", ssl.Certificate)
		}

		err = ioutil.WriteFile(ssl.Key, nsSecret.Fields[opts.Key.ValueFrom.SecretKeyRef.Key], 0600)
		if err != nil {
			return nil, fmt.Errorf("failed to write key to %s", ssl.Key)
		}

		if ssl.ClientCertificate != nil {
			err = ioutil.WriteFile(*ssl.ClientCertificate, nsSecret.Fields[opts.ClientCertficate.ValueFrom.SecretKeyRef.Key], 0644)
			if err != nil {
				return nil, fmt.Errorf("failed to write client cert to %s", *ssl.ClientCertificate)
			}
		}
	}

	return ssl, nil
}

/*
GetConf takes the router cache and returns a generated nginx configuration
*/
func GetConf(config *router.Config, cache *router.Cache) string {

	// Make sure we've converted the API Key to nginx format
	convertAPIKeyHeaderForNginx(config)

	tmplData := templateDataT{
		APIKeyHeader:          nginxAPIKeyHeader,
		Hosts:                 make(map[string]*hostT),
		Upstreams:             make(map[string]*upstreamT),
		Config:                config,
		DefaultLocationReturn: defaultReturnFromConfig(config),
	}

	// Create hostT for each host in each Namespace
	for _, ns := range cache.Namespaces {
		for hostName, opts := range ns.Hosts {
			// If hostT does not exist for hostname create one.
			if _, ok := tmplData.Hosts[hostName]; !ok {

				hostData := &hostT{
					HostOptions:          &opts,
					Locations:            make(map[string]*locationT),
					NeedsDefaultLocation: true,
				}

				// Convert ssl options
				if opts.SSLOptions != nil {

					// If ssl is disabled do not route pods/ns
					if !config.Nginx.SSLEnabled {
						log.Printf("  Nginx Config: Found ssl enabled hostname (%s) but ssl is disabled.\n", hostName)
						continue
					}

					sslOpts, err := processSSLOptions(ns.Name, hostName, opts.SSLOptions, cache, config)
					if err != nil {
						log.Printf("  Nginx Config: Invalid ssl options for host %s: %v\n", hostName, err)
						continue
					}
					hostData.SSL = sslOpts
				}

				tmplData.Hosts[hostName] = hostData
			} else {
				// Multiple namespace use the same host.
				// TODO: In the future merge hostOptions
			}
		}
	}

	// Generate upstreams
	for _, pod := range cache.Pods {
		podNs, ok := cache.Namespaces[pod.Namespace]
		if !ok {
			log.Printf("  Nginx Config: Missing namespace (%s) for pod %s\n", pod.Namespace, pod.Name)
			continue
		}

		for hostName := range podNs.Hosts {
			// Host always exists we just created it above
			host, ok := tmplData.Hosts[hostName]
			if !ok {
				log.Printf("  Nginx Config: Missing host config (%s) for pod %s\n", hostName, pod.Name)
				continue
			}

			for _, route := range pod.Routes {
				upstreamKey := hostName + route.Incoming.Path
				upstreamHash := fmt.Sprint(hash(upstreamKey))
				upstreamName := "upstream" + upstreamHash
				target := route.Outgoing.IP
				if route.Outgoing.Port != "80" && route.Outgoing.Port != "443" {
					target += ":" + route.Outgoing.Port
				}

				// Unset the need for a default location if necessary
				if host.NeedsDefaultLocation && route.Incoming.Path == "/" {
					host.NeedsDefaultLocation = false
				}

				location, ok := host.Locations[route.Incoming.Path]
				if !ok {
					// Calculate secret for location
					var locationSecret string
					secret, ok := cache.Secrets[pod.Namespace]
					if ok && secret.RoutingKey != nil {
						// Guaranteed to be an API Key so no need to double check
						locationSecret = base64.StdEncoding.EncodeToString(*secret.RoutingKey)
					}

					host.Locations[route.Incoming.Path] = &locationT{
						Namespace:  pod.Namespace,
						Secret:     locationSecret,
						Path:       route.Incoming.Path,
						Upstream:   upstreamName,
						TargetPath: route.Outgoing.TargetPath,
					}
				} else {
					// Location already exists

					// Check if location is in the same namespace
					if location.Namespace != pod.Namespace {
						log.Printf("  Nginx Config: Duplicate hostname and path for namespace:%s path:%s pod %s in namespace %s is duplicate.\n", location.Namespace, location.Path, pod.Name, pod.Namespace)
						// TODO: Better logging / handling of mis configuration

						// We cann't add pod host/path combitation if in different namespaces because secrets are per namespace
						// Move on to next route.
						continue
					}

					// Set targetPath for upstream if it's stil null
					// Note: If pods have different target paths the last pod sets the target path.
					if route.Outgoing.TargetPath != nil && location.TargetPath == nil {
						log.Printf("  Nginx Config: Inconsistent targetPath for pods for host:%s and path:%s new targetPath will be %s was nil\n", hostName, route.Incoming.Path, *route.Outgoing.TargetPath)
						location.TargetPath = route.Outgoing.TargetPath
					} else if location.TargetPath != nil && route.Outgoing.TargetPath != nil && *route.Outgoing.TargetPath != *location.TargetPath {
						log.Printf("  Nginx Config: Inconsistent targetPath for pods for host:%s and path:%s %s != %s\n", hostName, route.Incoming.Path, *route.Outgoing.TargetPath, *location.TargetPath)
					}
				}

				// Create or add to the upstreams
				if upstream, ok := tmplData.Upstreams[upstreamKey]; ok {
					// Upsteam already created
					upstream.Servers = append(upstream.Servers, &serverT{
						Pod:    pod,
						Target: target,
						Weight: route.Outgoing.Weight,
					})

					// Sort to make finding your pods in an upstream easier
					sort.Sort(upstream.Servers)

					if upstream.HealthCheck == nil && route.Outgoing.HealthCheck != nil {
						log.Printf("  Nginx Conf: Inconsistent HealthCheck for host:%s path:%s", hostName, route.Incoming.Path)
						upstream.HealthCheck = route.Outgoing.HealthCheck
					}
				} else {
					// Create new upstream
					tmplData.Upstreams[upstreamKey] = &upstreamT{
						Name:        upstreamName,
						Namespace:   pod.Namespace,
						Path:        route.Incoming.Path,
						HealthCheck: route.Outgoing.HealthCheck,
						Servers: []*serverT{
							&serverT{
								Pod:    pod,
								Target: target,
								Weight: route.Outgoing.Weight,
							},
						},
					}
				}

			}

		}
	}

	var doc bytes.Buffer

	// Useful for debugging
	if err := nginxTemplate.ExecuteTemplate(&doc, "nginx", tmplData); err != nil {
		log.Fatalf("Failed to write template %v", err)
	}

	return doc.String()
}

/*
hash creates a fnv hash for inputted string
*/
func hash(s string) uint32 {
	h := fnv.New32a()
	h.Write([]byte(s))
	return h.Sum32()
}

/*
convertAPIKeyHeaderForNginx converts the api key header to nginx compatible format
*/
func convertAPIKeyHeaderForNginx(config *router.Config) {
	if nginxAPIKeyHeader == "" {
		// Convert the API Key header to nginx
		nginxAPIKeyHeader = strings.ToLower(regexp.MustCompile("[^A-Za-z0-9]").ReplaceAllString(config.Nginx.APIKeyHeader, "_"))
	}
}
