package router

import (
	"k8s.io/client-go/kubernetes"
	api "k8s.io/client-go/pkg/api/v1"
	"log"
	"regexp"
	"strings"
)

const (
	hostnameRegexStr = "^(([a-zA-Z0-9]|[a-zA-Z0-9][a-zA-Z0-9\\-]*[a-zA-Z0-9])\\.)*([A-Za-z0-9]|[A-Za-z0-9][A-Za-z0-9\\-]*[A-Za-z0-9])$"
	ipRegexStr       = "^(([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])\\.){3}([0-9]|[1-9][0-9]|1[0-9]{2}|2[0-4][0-9]|25[0-5])$"
)

var hostnameRegex *regexp.Regexp
var ipRegex *regexp.Regexp

func init() {
	// Compile all regular expressions
	hostnameRegex = compileRegex(hostnameRegexStr)
	ipRegex = compileRegex(ipRegexStr)
}

/*
GetNamespaces returns all namespaces that match selector
*/
func GetNamespaces(config *Config, kubeClient *kubernetes.Clientset) (*api.NamespaceList, error) {
	// Query the initial list of Pods
	return kubeClient.Core().Namespaces().List(api.ListOptions{
		LabelSelector: config.NamespaceRoutableLabelSelector,
	})
}

/*
 Converts a Kubernetes namespace model to a dispatcher namespace model
*/
func ConvertNamespaceToModel(config *Config, namespace *api.Namespace) *Namespace {
	return &Namespace{
		Name:  namespace.Name,
		Hosts: GetHostsFromNamespace(config, namespace),
	}
}

/*
GetHostsFromNamespace returns all valid hosts from configured host annotation on Namespace
*/
func GetHostsFromNamespace(config *Config, namespace *api.Namespace) []string {
	var hosts []string

	annotation, ok := namespace.Annotations[config.NamespaceHostsAnnotation]
	if !ok {
		return hosts
	}

	// Process the routing hosts
	for _, host := range strings.Split(annotation, " ") {
		valid := hostnameRegex.MatchString(host)

		if !valid {
			valid = ipRegex.MatchString(host)

			if !valid {
				log.Printf("Namespace (%s) host issue: %s (%s) is not a valid hostname/ip\n", namespace.Name, config.NamespaceHostsAnnotation, host)
				continue
			}
		}

		// Record the host
		hosts = append(hosts, host)
	}
	return hosts
}

// compileRegex returns a regex object from a regex string
func compileRegex(regexStr string) *regexp.Regexp {
	compiled, err := regexp.Compile(regexStr)

	if err != nil {
		log.Fatalf("Failed to compile regular expression (%s): %v\n", regexStr, err)
	}

	return compiled
}
