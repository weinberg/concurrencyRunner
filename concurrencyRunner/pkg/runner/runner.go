package runner

import (
	"bufio"
	"fmt"
	"github.com/google/go-dap"
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
	Client   *client.Client
	Cmd      *exec.Cmd
	Type     config.AdapterEnum
	Url      string
	ThreadId int
	// Breakpoints stores the responses from setBreakpoints so we can
	// report on which breakpoint was hit
	Breakpoints map[int]dap.Breakpoint
	// Instance is the instance from the config file
	Instance config.Instance
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
	defer r.Cleanup()

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
 * Cleanup
 ***************************************************/

func (r *Runtime) Cleanup() (err error) {
	for _, ia := range r.InstanceAdapters {
		err = ia.Cmd.Process.Kill()
		if err != nil {
			fmt.Printf("Error killing instance '%s': %s\n", ia.Instance.Id, err)
		}
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
		<-cl.Events

		// delve sends output event, ignore it
		<-cl.Events

		// Configuration Done Response
		<-cl.Responses

		// Get threads
		err = cl.ThreadsRequest()
		if err != nil {
			return err
		}

		response := <-cl.Responses
		fmt.Printf("%v+\n", response)
		/*
			if len(threads.Body.Threads) > 1 {
				return fmt.Errorf("client has more than 1 thread: %d", len(threads.Body.Threads))
			}
			if err != nil {
				return err
			}

			instance.ThreadId = threads.Body.Threads[0].Id
		*/
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
			//var i int
			for _, response := range breakpointsResponse.Body.Breakpoints {
				if response.Verified == false {
					return fmt.Errorf("breakpoint could not be set in file '%s' at line '%d': %s",
						file, response.Line, breakpointsResponse.Body.Breakpoints[0].Message)
				}
				r.InstanceAdapters[instanceId].Breakpoints[response.Id] = response
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
		switch action.Type {
		case config.ActionTypeRun:
			r.actionRun(action)
		case config.ActionTypePause:
			r.actionPause(action)
		case config.ActionTypeContinue:
			r.actionContinue(action)
		case config.ActionTypeSleep:
			r.actionSleep(action)
		}
	}
	return
}

func (r *Runtime) drainEvents() {
	for _, ia := range r.InstanceAdapters {
		ia.Client.ReadMessage()

	}
}

func (r *Runtime) actionSleep(action config.Action) (err error) {
	var suffix string = "s"
	if action.SleepDuration == 1 {
		suffix = ""
	}
	fmt.Printf("Instance All: SLEEPING %d second%s\n", action.SleepDuration, suffix)

	time.Sleep(action.SleepDuration * time.Second)

	return
}

func (r *Runtime) actionContinue(action config.Action) (err error) {
	ia := r.InstanceAdapters[action.InstanceId]
	cl := ia.Client

	err = cl.ContinueRequest(ia.ThreadId)
	if err != nil {
		return err
	}

	fmt.Printf("Instance '%s': CONTINUE\n", action.InstanceId)

	return
}

func (r *Runtime) actionPause(action config.Action) (err error) {
	ia := r.InstanceAdapters[action.InstanceId]
	cl := ia.Client

	event, err := cl.ReadStoppedEvent()
	if err != nil {
		return err
	}

	for _, id := range event.Body.HitBreakpointIds {
		fmt.Printf("Instance '%s': PAUSE at file '%s', line: %d\n",
			action.InstanceId, ia.Breakpoints[id].Source.Path, ia.Breakpoints[id].Line)
	}

	return
}

func (r *Runtime) actionRun(action config.Action) (err error) {
	ia := r.InstanceAdapters[action.InstanceId]
	cl := ia.Client
	err = cl.ContinueRequest(ia.ThreadId)
	if err != nil {
		return err
	}

	_, err = cl.ReadContinueResponse()
	if err != nil {
		return err
	}

	fmt.Printf("Instance '%s': RUN\n", action.InstanceId)

	return
}

/****************************************************
 * DAP Clients
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

	instanceAdapter.Instance = instance
	r.InstanceAdapters[instance.Id] = instanceAdapter

	return nil
}

func (r *Runtime) LaunchDelveAdapter(instance config.Instance) (instanceAdapter *InstanceAdapter, err error) {
	instanceAdapter = &InstanceAdapter{
		Type:        config.AdapterDelve,
		Breakpoints: map[int]dap.Breakpoint{},
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
