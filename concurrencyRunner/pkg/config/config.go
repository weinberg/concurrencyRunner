package config

import (
	"encoding/json"
	"github.com/gookit/color"
	"io/ioutil"
	"os"
	"time"
)

type Instance struct {
	Id       string
	Name     string
	Adapter  AdapterEnum
	Program  string
	Env      string
	Cwd      string
	SrcRoot  string
	Args     []string
	OutputBg color.Color
}

type Action struct {
	InstanceId    string
	Type          ActionTypeEnum `json:"action,omitempty"`
	File          string
	TargetComment string
	SleepDuration time.Duration `json:"duration,omitempty"`
}

type Config struct {
	Instances []Instance
	Sequence  []Action
}

func ReadConfigFile(path string) (config *Config, err error) {
	jsonFile, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer jsonFile.Close()
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(byteValue, &config)

	return
}
