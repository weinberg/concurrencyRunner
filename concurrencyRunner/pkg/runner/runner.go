package runner

import (
	"bufio"
	"fmt"
	"github.com/weinberg/concurrencyRunner/pkg/client"
	"github.com/weinberg/concurrencyRunner/pkg/config"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type InstanceAdapter struct {
	Client *client.Client
	Cmd    *exec.Cmd
	Type   config.AdapterEnum
	Url    string
}

type DelveAdapterData struct {
	Hostname string
	NextPort int
}

type Runtime struct {
	// maps instance Id from config file to instance runtime data
	InstanceAdapters map[string]*InstanceAdapter
	// adapter specific data
	DelveAdapterData *DelveAdapterData
}

func NewRuntime() *Runtime {
	return &Runtime{
		InstanceAdapters: make(map[string]*InstanceAdapter),
	}
}

func NewDelveAdapterData() *DelveAdapterData {
	return &DelveAdapterData{
		Hostname: "localhost",
		NextPort: 59000,
	}
}

func Run(config *config.Config) (err error) {
	r := NewRuntime()
	err = r.LaunchClients(config)
	if err != nil {
		return
	}
	defer r.ShutdownClients()

	err = r.SetupInstances(config)
	if err != nil {
		return err
	}

	err = r.RunSequence(config)
	if err != nil {
		return err
	}

	return
}

/****************************************************
 * Setup
 ***************************************************/

func (r *Runtime) SetupInstances(c *config.Config) (err error) {
	err = r.SetBreakpoints(c)
	if err != nil {
		return err
	}

	err = r.CompleteSetup(c)
	return
}

func (r *Runtime) CompleteSetup(c *config.Config) (err error) {
	for _, instance := range r.InstanceAdapters {
		cl := instance.Client
		// send configuration done
		err = cl.ConfigurationDoneRequest()
		if err != nil {
			return err
		}

		// get stopped event - expected after launch
		_, err = cl.ReadStoppedEvent()
		if err != nil {
			return err
		}

		// delve sends output event, ignore it
		_, err = cl.ReadMessage()
		if err != nil {
			return err
		}

		// Configuration Done Response
		_, err = cl.ReadConfigurationDoneResponse()
		if err != nil {
			return err
		}
	}

	return
}

// SetBreakpoints sets breakpoints in all instances
func (r *Runtime) SetBreakpoints(c *config.Config) (err error) {
	// map of instanceId -> map of file -> lines
	breakpoints := make(map[string]map[string][]int)

	for _, action := range c.Sequence {
		if action.Type != config.ActionTypePause {
			continue
		}

		line, err := findTargetComment(action.File, action.TargetComment)
		if err != nil {
			return err
		}

		absFilepath, err := filepath.Abs(action.File)
		if err != nil {
			return err
		}

		if _, ok := breakpoints[action.InstanceId]; !ok {
			breakpoints[action.InstanceId] = make(map[string][]int)
		}

		if _, ok := breakpoints[action.InstanceId][absFilepath]; !ok {
			breakpoints[action.InstanceId][absFilepath] = []int{}
		}
		breakpoints[action.InstanceId][absFilepath] = append(breakpoints[action.InstanceId][absFilepath], line)
	}

	for instanceId, bpData := range breakpoints {
		c := r.InstanceAdapters[instanceId].Client
		for file, lines := range bpData {
			err := c.SetBreakpointsRequestWithArgs(file, lines, nil, nil, nil)
			if err != nil {
				return err
			}
			breakpointsResponse, err := c.ReadSetBreakpointsResponse()
			if err != nil {
				return err
			}
			breakpoints := breakpointsResponse.Body.Breakpoints
			for _, response := range breakpointsResponse.Body.Breakpoints {
				if response.Verified == false {
					return fmt.Errorf("breakpoint could not be set in file '%s' at line '%d': %s",
						file, response.Line, breakpointsResponse.Body.Breakpoints[0].Message)
				}
			}
			if len(breakpoints) != len(lines) {
				return fmt.Errorf("all breakpoints could not be set in file '%s' at lines %v",
					file, lines)
			}
		}
	}
	return
}

// findTargetComments searches file for `comments` and returns the line number
func findTargetComment(file string, comment string) (int, error) {
	f, err := os.Open(file)
	if err != nil {
		return 0, err
	}
	defer func(f *os.File) {
		err := f.Close()
		if err != nil {
			log.Fatal(err)
		}
	}(f)

	// Splits on newlines by default.
	scanner := bufio.NewScanner(f)

	line := 1
	for scanner.Scan() {
		if strings.Contains(scanner.Text(), comment) {
			return line, nil
		}

		line++
	}

	if err := scanner.Err(); err != nil {
		return 0, err
	}

	return 0, fmt.Errorf("TargetComment '%s' not found in file '%s'\n", comment, file)
}

/****************************************************
 * Run Sequence
 ***************************************************/

func (r *Runtime) RunSequence(c *config.Config) (err error) {
	for _, action := range c.Sequence {
		fmt.Printf("%v", action)
		switch action.Type {
		case config.ActionTypeRun:
			{
				r.runAction(action)

			}
		case config.ActionTypePause:
			{

			}
		case config.ActionTypeContinue:
			{

			}
		}
	}
	return
}

func (r *Runtime) runAction(action config.Action) {
}

/****************************************************
 * Setup
 ***************************************************/

// LaunchClients starts a DAP client for each instance
func (r *Runtime) LaunchClients(config *config.Config) (err error) {
	for _, instance := range config.Instances {
		cl, err := r.LaunchClient(instance)
		if err != nil {
			return err
		}
		r.InstanceAdapters[instance.Id].Client = cl
	}
	return
}

func (r *Runtime) LaunchClient(instance config.Instance) (*client.Client, error) {
	// each client requires its own DAP
	err := r.LaunchDAP(instance)
	if err != nil {
		return nil, err
	}

	// create client
	clientUrl := r.InstanceAdapters[instance.Id].Url
	cl, err := client.NewClient(clientUrl)
	if err != nil {
		return nil, err
	}

	// initialize
	_, err = cl.Initialize()
	if err != nil {
		return nil, err
	}

	// extract env vars
	envVars := make(map[string]string)
	envKvs := strings.Split(instance.Env, ";")
	for _, envKv := range envKvs {
		kv := strings.Split(envKv, "=")
		if kv[0] == "" || kv[1] == "" {
			return nil, fmt.Errorf("invalid env string: %s", instance.Env)
		}
		envVars[kv[0]] = kv[1]
	}

	// launch debugee
	err = cl.LaunchRequestWithArgs(map[string]interface{}{
		"request":     "launch",
		"mode":        "debug",
		"program":     instance.Program,
		"stopOnEntry": true,
		"env":         envVars,
		"dlvCwd":      instance.Cwd,
	})
	// initialized event
	_, err = cl.ReadInitializedEvent()
	if err != nil {
		return nil, err
	}

	// launch response comes after initialized event
	_, err = cl.ReadLaunchResponse()
	if err != nil {
		return nil, err
	}

	return cl, nil
}

func (r *Runtime) ShutdownClients() {
	for _, ia := range r.InstanceAdapters {
		if err := ia.Cmd.Process.Kill(); err != nil {
			log.Fatal("failed to kill process: ", err)
		}
	}
}

func (r *Runtime) LaunchDAP(instance config.Instance) (err error) {
	var instanceAdapter *InstanceAdapter
	switch instance.Adapter {
	case config.AdapterDelve:
		{
			instanceAdapter, err = r.LaunchDelveAdapter(instance)
			if err != nil {
				return err
			}
		}
	}

	r.InstanceAdapters[instance.Id] = instanceAdapter

	return nil
}

func (r *Runtime) LaunchDelveAdapter(instance config.Instance) (instanceAdapter *InstanceAdapter, err error) {
	instanceAdapter = &InstanceAdapter{
		Type: config.AdapterDelve,
	}
	if r.DelveAdapterData == nil {
		r.DelveAdapterData = NewDelveAdapterData()
	}
	instanceAdapter.Url = fmt.Sprintf("%s:%d", r.DelveAdapterData.Hostname, r.DelveAdapterData.NextPort)
	r.DelveAdapterData.NextPort++

	cmd := exec.Command("dlv", "dap", "--wd", instance.Cwd, "--listen", instanceAdapter.Url)
	err = cmd.Start()
	if err != nil {
		log.Fatalf("LaunchDelveAdapter: cannot start delve: %s", err.Error())
	}

	instanceAdapter.Cmd = cmd

	// delay briefly to let adapter open port
	time.Sleep(100 * time.Millisecond)

	return
}
