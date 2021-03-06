package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/TV4/env"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
	yaml "gopkg.in/yaml.v2"
)

// Update contains everything that DuckDNS will need to update a record
type Update struct {
	Token string   `yaml:"token"`
	Names []string `yaml:"domains"`
}

// CLIOptions are to set things via CLI
type CLIOptions struct {
	Debug bool
	File  string
	Token string
	Names []string
}

// Valid checks that all parameters are set for an update
func (u *Update) Valid() bool {
	if len(u.Names) > 0 && u.Token != "" {
		return true
	}
	return false
}

// GetConfigCLI sets the arguments for an update if they have been passed in on
// the CLI
func getConfigCLI(c CLIOptions) Update {
	var u Update

	u.Token = c.Token
	logrus.Debugf("Set token from CLI to %s", c.Token)
	u.Names = c.Names
	logrus.Debugf("Set names from CLI to %s", strings.Join(c.Names, ", "))

	return u
}

// GetConfigFile reads the config for DuckDNS
func getConfigFile(existing *Update, file string) {

	var update Update

	yamlFile, err := ioutil.ReadFile(file)
	if err != nil {
		logrus.WithError(err).Debug("error reading file")
		return
	}
	err = yaml.Unmarshal(yamlFile, &update)
	if err != nil {
		logrus.WithError(err).Debug("error unmarshaling YAML file")
		return
	}

	// Set the token if it's not empty and doesn't already exist
	if update.Token == "" {
		logrus.Debugf("the token is empty after trying to parse %s", file)
	} else if existing.Token == "" {
		existing.Token = update.Token
	}

	// Set names to if they exist and value is not already set
	if len(update.Names) == 0 {
		logrus.Debugf("no names/subdomains specified to update from %s", file)
	} else if len(existing.Names) == 0 {
		existing.Names = update.Names
	}

}

// GetConfigEnv is for reading items out of the environment if you didn't want
// to set them on the CLI
func getConfigEnv(u *Update) {
	token := env.String("DUCK_TOKEN", "")
	name := env.String("DUCK_NAMES", "")

	// Set the token if not already set
	if u.Token == "" {
		u.Token = token
		logrus.Debugf("Set token from environment to %s", token)
	}

	if len(u.Names) == 0 && name != "" {
		// support DUCK_NAME="domain1 domain2" from the environment
		u.Names = strings.Split(name, " ")

		logrus.Debugf("Set names from environment to %s",
			strings.Join(u.Names, ", "))
	}
}

func makeUpdate(update Update) error {
	logrus.Debugf("Dumping update params: %#v", update)
	if !update.Valid() {
		logrus.Fatal("Arguments not set for update!")
		os.Exit(1)
	}
	var errs []string
	stub := "https://www.duckdns.org/update?domains="
	tokenStub := "&token="
	ipStub := "&ip="

	for _, v := range update.Names {

		url := fmt.Sprintf("%s%s%s%s%s", stub, v, tokenStub, update.Token, ipStub)
		logrus.Debugf("Update string: %s", url)
		res, err := http.Get(url)
		if err != nil {
			errs = append(errs, err.Error())
			logrus.WithError(err).Error("Error contacting DuckDNS server")
			continue
		}

		bodyBytes, err := ioutil.ReadAll(res.Body)
		if err != nil {
			errs = append(errs, err.Error())
			logrus.WithError(err).Error("Error reading body response")
			continue
		}
		res.Body.Close()

		if strings.Contains(string(bodyBytes), "KO") {
			errs = append(errs, fmt.Sprintf("Error updating %s with DuckDNS", v))
			continue
		}

		logrus.Debugf("updated DuckDNS for name %s", v)

	}

	if len(errs) != 0 {
		return errors.New(strings.Join(errs, "\n"))
	}

	return nil
}

func main() {
	var cli CLIOptions

	pflag.BoolVarP(&cli.Debug, "debug", "d", false, "Use debug mode")
	pflag.StringVarP(&cli.File, "config", "c", "duckdns.yaml",
		"Config file location")
	pflag.StringSliceVarP(&cli.Names, "names", "n", nil,
		"Names to update with DuckDNS. Just the subdomain section. "+
			"Use the flag multiple times to set multiple values.")
	pflag.StringVarP(&cli.Token, "token", "t", "",
		"Token for updating DuckDNS")

	pflag.Parse()

	if cli.Debug {
		logrus.SetLevel(logrus.DebugLevel)
	}
	logrus.Debugf("Logging level: %s", logrus.GetLevel().String())

	// CLI vars
	update := getConfigCLI(cli)

	// Set things that weren't set by the CLI
	if !update.Valid() {
		getConfigEnv(&update)
	}

	// File vars
	if !update.Valid() {
		getConfigFile(&update, cli.File)
	}

	if err := makeUpdate(update); err != nil {
		logrus.WithError(err).Fatal("error updating IP address")
		os.Exit(1)
	}
	logrus.Debug("IP address updated successfully")
}
