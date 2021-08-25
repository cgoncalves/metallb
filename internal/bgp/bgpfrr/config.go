package bgpfrr

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"text/template"
)

// TODO configFileName will need to be set to the filename of the FRR configuration
// file in the container.
var configFileName = "frr.conf"

const configTemplate = `
log stdout {{.Loglevel}}
hostname {{.Hostname}}

{{range .Routers -}}
router bgp {{.MyASN}}
  no bgp default ipv4-unicast
  bgp router-id {{.RouterId}}
{{range .Neighbors }}
  neighbor {{.Addr}} remote-as {{.ASN}}
{{- end }}
{{range $n := .Neighbors -}}
{{range .Advertisements }}
  address-family {{.Version}} unicast
    neighbor {{$n.Addr}} activate
    network {{.Prefix}}
  exit-address-family
{{- end}}
{{end}}
{{end}}
`

type frrConfig struct {
	Loglevel string
	Hostname string
	Routers  map[string]*routerConfig
}

type routerConfig struct {
	MyASN     uint32
	RouterId  string
	Neighbors map[string]*neighborConfig
}

type neighborConfig struct {
	ASN            uint32
	Addr           string
	Advertisements map[string]*advertisementConfig
}

type advertisementConfig struct {
	Version string
	Prefix  string
}

// routerName() defines the format of the key of the "Routers" map in the
// frrConfig struct.
func routerName(srcAddr string, myASN uint32) string {
	return fmt.Sprintf("%d@%s", myASN, srcAddr)
}

// neighborName() defines the format of key of the 'Neighbors' map in the
// routerConfig struct.
func neighborName(peerAddr string, ASN uint32) string {
	return fmt.Sprintf("%d@%s", ASN, peerAddr)
}

// templateConfig uses the template library to template
// 'globalConfigTemplate' using 'data'.
func templateConfig(data interface{}) (string, error) {
	t, err := template.New("FRR Config Template").Parse(configTemplate)
	if err != nil {
		return "", err
	}

	var b bytes.Buffer
	err = t.Execute(&b, data)

	return b.String(), err
}

// writeConfigFile writes the FRR configuration file (represented as a string)
// to 'filename'.
func writeConfig(config string, filename string) error {
	return ioutil.WriteFile(filename, []byte(config), 0644)
}

// reloadConfig requests that FRR reloads the configuration file. This is
// called after updating the configuration.
func reloadConfig() error {
	// TODO
	return nil
}

func configFRR(config *frrConfig) error {
	filename, found := os.LookupEnv("FRR_CONFIG_FILE")
	if found {
		configFileName = filename
	}

	configString, err := templateConfig(config)
	if err != nil {
		return err
	}

	err = writeConfig(configString, configFileName)
	if err != nil {
		return err
	}

	err = reloadConfig()
	if err != nil {
		return err
	}

	return nil
}