package main

import (
	"fmt"
	"github.com/weinberg/concurrencyLab/pkg/client"
	"os"
	"time"
)

// A sample message exchange copied from
// https://github.com/go-delve/delve/blob/22fd222c0a4c705886512dcb1305125492d55d4a/service/dap/server_test.go
//
// "TestLaunchStopOnEntry emulates the message exchange that can be observed with
// VS Code for the most basic launch debug session with "stopOnEntry" enabled:
// - User selects "Start Debugging":  1 >> initialize
//                                 :  1 << initialize
//                                 :  2 >> launch
//                                 :    << initialized event
//                                 :  2 << launch
//                                 :  3 >> setBreakpoints (empty)
//                                 :  3 << setBreakpoints
//                                 :  4 >> setExceptionBreakpoints (empty)
//                                 :  4 << setExceptionBreakpoints
//                                 :  5 >> configurationDone
// - Program stops upon launching  :    << stopped event
//                                 :  5 << configurationDone
//                                 :  6 >> threads
//                                 :  6 << threads (Dummy)
//                                 :  7 >> threads
//                                 :  7 << threads (Dummy)
//                                 :  8 >> stackTrace
//                                 :  8 << error (Unable to produce stack trace)
//                                 :  9 >> stackTrace
//                                 :  9 << error (Unable to produce stack trace)
// - User evaluates bad expression : 10 >> evaluate
//                                 : 10 << error (unable to find function context)
// - User evaluates good expression: 11 >> evaluate
//                                 : 11 << evaluate
// - User selects "Continue"       : 12 >> continue
//                                 : 12 << continue
// - Program runs to completion    :    << terminated event
//                                 : 13 >> disconnect
//                                 :    << output event (Process exited)
//                                 :    << output event (Detaching)
//                                 : 13 << disconnect
// "

func do(clientURL, program, file, cwd string, lines []int, envVars map[string]string, clientNo int) (*client.Client, error) {
	// create client
	client, err := client.NewClient(clientURL)
	if err != nil {
		return nil, err
	}

	// initialize
	_, err = client.Initialize()
	if err != nil {
		return nil, err
	}

	// launch debugee
	err = client.LaunchRequestWithArgs(map[string]interface{}{
		"request":     "launch",
		"mode":        "debug",
		"program":     program,
		"stopOnEntry": true,
		"env":         envVars,
		"dlvCwd":      cwd,
	})
	// initialized event
	_, err = client.ReadInitializedEvent()
	if err != nil {
		return nil, err
	}

	// launch response comes after initialized event
	_, err = client.ReadLaunchResponse()
	if err != nil {
		return nil, err
	}

	// set breakpoints if there are any
	if lines != nil && len(lines) > 0 {
		err = client.SetBreakpointsRequestWithArgs(file, lines, nil, nil, nil)
		if err != nil {
			return nil, err
		}
		breakpointsResponse, err := client.ReadSetBreakpointsResponse()
		if err != nil {
			return nil, err
		}
		breakpoints := breakpointsResponse.Body.Breakpoints
		if len(breakpoints) != len(lines) {
			return nil, fmt.Errorf("Breakpoints could not be set")
		}
		if breakpointsResponse.Body.Breakpoints[0].Verified == false {
			return nil, fmt.Errorf("Breakpoint could not be set: %s", breakpointsResponse.Body.Breakpoints[0].Message)
		}
	}

	// send configuration done
	err = client.ConfigurationDoneRequest()
	if err != nil {
		return nil, err
	}

	// get stopped event - expected after launch
	_, err = client.ReadStoppedEvent()
	if err != nil {
		return nil, err
	}

	// delve sends output event, ignore it
	_, err = client.ReadMessage()
	if err != nil {
		return nil, err
	}

	// Configuration Done Response
	_, err = client.ReadConfigurationDoneResponse()
	if err != nil {
		return nil, err
	}

	return client, nil
}

// readModifyWrite demonstrates the lost-write concurrency bug using two clients
// client1 - BEGIN, READ, MODIFY, breakpoint...
// client2 - BEGIN, READ, MODIFY, WRITE, COMMIT
// client1 - ...continue, WRITE, COMMIT
func readModifyWrite() error {
	// launch two clients
	envVars := make(map[string]string)
	envVars["DATABASE_URL"] = "postgres://postgres:pass@localhost:5433/postgres"
	clientUrl := "localhost:59000"
	program := "github.com/weinberg/concurrencyLabExampleTargets/cmd/readModifyWrite"
	file := "/Users/josh/dev/concurrencyLabExampleTargets/cmd/readModifyWrite/main.go"
	cwd := "/Users/josh/dev/concurrencyLabExampleTargets"
	var clientNo int = 0
	lines := []int{41}
	client1, err := do(clientUrl, program, file, cwd, lines, envVars, clientNo)
	if err != nil {
		return err
	}
	clientUrl = "localhost:59001"
	lines = []int{}
	clientNo++
	client2, err := do(clientUrl, program, file, cwd, lines, envVars, clientNo)
	if err != nil {
		return err
	}

	// client1 continue to start execution
	err = client1.ThreadsRequest()
	if err != nil {
		return err
	}
	threads1, err := client1.ReadThreadsResponse()
	if len(threads1.Body.Threads) > 1 {
		return fmt.Errorf("Client 1 has unexpected number of threads ()", len(threads1.Body.Threads))
	}

	err = client1.ContinueRequest(threads1.Body.Threads[0].Id)
	if err != nil {
		return err
	}

	_, err = client1.ReadContinueResponse()
	if err != nil {
		return err
	}

	_, err = client1.ReadStoppedEvent()
	if err != nil {
		return err
	}

	// client 1 stopped
	// sleep just for show... it's not required to demonstrate this bug
	time.Sleep(1 * time.Second)

	// client 2 run til completion
	err = client2.ThreadsRequest()
	if err != nil {
		return err
	}
	threads2, err := client2.ReadThreadsResponse()
	if len(threads2.Body.Threads) > 1 {
		return fmt.Errorf("Client 2 has unexpected number of threads ()", len(threads2.Body.Threads))
	}

	err = client2.ContinueRequest(threads2.Body.Threads[0].Id)
	if err != nil {
		return err
	}

	_, err = client2.ReadContinueResponse()
	if err != nil {
		return err
	}

	_, err = client2.ReadTerminatedEvent()
	if err != nil {
		return err
	}

	// continue client 1
	err = client1.ContinueRequest(threads1.Body.Threads[0].Id)
	if err != nil {
		return err
	}

	_, err = client1.ReadContinueResponse()
	if err != nil {
		return err
	}

	_, err = client1.ReadTerminatedEvent()
	if err != nil {
		return err
	}

	return nil
}

func main() {
	err := readModifyWrite()
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		os.Exit(1)
	}

	fmt.Printf("Exiting")
}
