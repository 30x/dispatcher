package nginx

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"github.com/30x/dispatcher/router"
	"hash/fnv"
	"log"
	"regexp"
	"sort"
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
    {{range $server := $upstream.Servers -}}
    server {{$server.Target}};
    {{- end}}
  }
  {{end -}}

  {{range $ns, $server := .Hosts}}
  server {
    listen {{$.Config.Nginx.Port}};
    server_name{{range $host, $opts := $server.HostNames}} {{$host}}{{end}};

    {{if $server.NeedsDefaultLocation -}}
    {{template "default-location" .}}
    {{- end -}}

    {{range $path, $location := $server.Locations -}}
    location {{$path}} {
      {{if ne $server.Secret "" -}}
      # Check the Routing API Key (namespace: {{$ns}})
      if ($http_{{$.APIKeyHeader}} != "{{$server.Secret}}") {
        return 403;
      }
      {{- end}}
      #Upstream {{$location.Upstream}}
      proxy_pass http://{{$location.Upstream}};
    }

    {{end}}
  }
  {{end}}

  {{template "default-server" .}}
}
`

	partialsTmpl = `
{{define "base-config" -}}
events {
  worker_connections 1024;
}
{{- end}}

{{define "default-server" -}}
  # Default server that will just close the connection as if there was no server available
  server {
    listen {{.Config.Nginx.Port}} default_server;
    return 444;
  }
{{- end}}

{{define "default-location" -}}
    # Here to avoid returning the nginx welcome page for servers that do not have a "/" location.  (Issue #35)
    location / {
      return 404;
    }
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
  proxy_set_header Host $http_host;
  proxy_set_header Upgrade $http_upgrade;
{{- end}}
`
)

type hostT struct {
	HostNames            map[string]*router.HostOptions
	Locations            map[string]*locationT
	Secret               string
	NeedsDefaultLocation bool
}

type locationT struct {
	Path     string
	Upstream string
}

type upstreamT struct {
	Namespace string
	Name      string
	Path      string
	Servers   serversT
}

type serverT struct {
	Pod    *router.PodWithRoutes
	Target string
}

type templateDataT struct {
	APIKeyHeader string
	Hosts        map[string]*hostT
	Upstreams    map[string]*upstreamT
	Config       *router.Config
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

/*
GetConf takes the router cache and returns a generated nginx configuration
*/
func GetConf(config *router.Config, cache *router.Cache) string {

	// Make sure we've converted the API Key to nginx format
	convertAPIKeyHeaderForNginx(config)

	tmplData := templateDataT{
		APIKeyHeader: nginxAPIKeyHeader,
		Hosts:        make(map[string]*hostT),
		Upstreams:    make(map[string]*upstreamT),
		Config:       config,
	}

	// Create hosts from Namespaces
	for _, ns := range cache.Namespaces {
		var locationSecret string
		secret, ok := cache.Secrets[ns.Name]
		if ok {
			// There is guaranteed to be an API Key so no need to double check
			locationSecret = base64.StdEncoding.EncodeToString(secret.Data)
		}

		host := hostT{
			HostNames:            make(map[string]*router.HostOptions),
			Locations:            make(map[string]*locationT),
			Secret:               locationSecret,
			NeedsDefaultLocation: true,
		}

		for hostName, opts := range ns.Hosts {
			host.HostNames[hostName] = &opts
		}
		tmplData.Hosts[ns.Name] = &host
	}

	// Generate upstreams
	for _, pod := range cache.Pods {
		host, ok := tmplData.Hosts[pod.Namespace]
		if !ok {
			log.Printf("  Nginx Config: Missing host/namespace for pod %s\n", pod.Name)
		}

		for _, route := range pod.Routes {
			upstreamKey := pod.Namespace + route.Incoming.Path
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

			_, ok := host.Locations[route.Incoming.Path]
			if !ok {
				host.Locations[route.Incoming.Path] = &locationT{
					Path:     route.Incoming.Path,
					Upstream: upstreamName,
				}
			}

			// Create or add to the upstreams
			if upstream, ok := tmplData.Upstreams[upstreamKey]; ok {
				// Upsteam already created
				upstream.Servers = append(upstream.Servers, &serverT{
					Pod:    pod,
					Target: target,
				})

				// Sort to make finding your pods in an upstream easier
				sort.Sort(upstream.Servers)
			} else {
				// Create new upstream
				tmplData.Upstreams[upstreamKey] = &upstreamT{
					Name:      upstreamName,
					Namespace: pod.Namespace,
					Path:      route.Incoming.Path,
					Servers: []*serverT{
						&serverT{
							Pod:    pod,
							Target: target,
						},
					},
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
