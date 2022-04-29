package client

import (
	"bufio"
	"encoding/json"
	"fmt"
	"github.com/google/go-dap"
	"net"
	"path/filepath"
)

// This client code is from the Delve test suite
// @see https://github.com/go-delve/delve/blob/v1.8.2/service/dap/daptest/client.go#L256

// Client is a bugs service client that uses Debug Adaptor Protocol.
// It does not (yet?) implement service.Client interface.
// All client methods are synchronous.
type Client struct {
	conn   net.Conn
	reader *bufio.Reader
	// seq is used to track the sequence number of each
	// requests that the client sends to the readModifyWrite
	seq                int
	initializeResponse dap.Message
}

// NewClient creates a new Client over a TCP connection.
// Call Close() to close the connection.
func NewClient(addr string) (client *Client, err error) {
	fmt.Println("Connecting to readModifyWrite at:", addr)
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("dialing %s", err)
	}
	return NewClientFromConn(conn), nil
}

// NewClientFromConn creates a new Client with the given TCP connection.
// Call Close to close the connection.
func NewClientFromConn(conn net.Conn) *Client {
	c := &Client{conn: conn, reader: bufio.NewReader(conn)}
	c.seq = 1 // match VS Code numbering
	return c
}

// Close closes the client connection.
func (c *Client) Close() {
	c.conn.Close()
}

func (c *Client) send(request dap.Message) error {
	return dap.WriteProtocolMessage(c.conn, request)
}

func (c *Client) ReadMessage() (dap.Message, error) {
	return dap.ReadProtocolMessage(c.reader)
}

// InitializeRequest sends an 'initialize' request.
func (c *Client) InitializeRequest() error {
	request := &dap.InitializeRequest{Request: *c.newRequest("initialize")}
	request.Arguments = dap.InitializeRequestArguments{
		AdapterID:                    "concurrency-lab",
		PathFormat:                   "path",
		LinesStartAt1:                true,
		ColumnsStartAt1:              true,
		SupportsVariableType:         true,
		SupportsVariablePaging:       true,
		SupportsRunInTerminalRequest: true,
		Locale:                       "en-us",
	}
	return c.send(request)
}

func (c *Client) Initialize() (dap.Message, error) {
	err := c.InitializeRequest()
	if err != nil {
		return nil, err
	}

	response, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}

	c.initializeResponse = response
	return response, nil
}

// InitializeRequestWithArgs sends an 'initialize' request with specified arguments.
func (c *Client) InitializeRequestWithArgs(args dap.InitializeRequestArguments) {
	request := &dap.InitializeRequest{Request: *c.newRequest("initialize")}
	request.Arguments = args
	c.send(request)
}

func toRawMessage(in interface{}) json.RawMessage {
	out, _ := json.Marshal(in)
	return out
}

// LaunchRequest sends a 'launch' request with the specified args.
func (c *Client) LaunchRequest(mode, program string, stopOnEntry bool) {
	request := &dap.LaunchRequest{Request: *c.newRequest("launch")}
	request.Arguments = toRawMessage(map[string]interface{}{
		"request":     "launch",
		"mode":        mode,
		"program":     program,
		"stopOnEntry": stopOnEntry,
	})
	c.send(request)
}

// LaunchRequestWithArgs takes a map of untyped implementation-specific
// arguments to send a 'launch' request. This version can be used to
// test for values of unexpected types or unspecified values.
func (c *Client) LaunchRequestWithArgs(arguments map[string]interface{}) error {
	request := &dap.LaunchRequest{Request: *c.newRequest("launch")}
	request.Arguments = toRawMessage(arguments)
	return c.send(request)
}

// AttachRequest sends an 'attach' request with the specified
// arguments.
func (c *Client) AttachRequest(arguments map[string]interface{}) {
	request := &dap.AttachRequest{Request: *c.newRequest("attach")}
	request.Arguments = toRawMessage(arguments)
	c.send(request)
}

// DisconnectRequest sends a 'disconnect' request.
func (c *Client) DisconnectRequest() {
	request := &dap.DisconnectRequest{Request: *c.newRequest("disconnect")}
	c.send(request)
}

// DisconnectRequestWithKillOption sends a 'disconnect' request with an option to specify
// `terminateDebuggee`.
func (c *Client) DisconnectRequestWithKillOption(kill bool) {
	request := &dap.DisconnectRequest{Request: *c.newRequest("disconnect")}
	request.Arguments.TerminateDebuggee = kill
	c.send(request)
}

// SetBreakpointsRequest sends a 'setBreakpoints' request.
func (c *Client) SetBreakpointsRequest(file string, lines []int) {
	c.SetBreakpointsRequestWithArgs(file, lines, nil, nil, nil)
}

// SetBreakpointsRequestWithArgs sends a 'setBreakpoints' request with an option to
// specify conditions, hit conditions, and log messages.
func (c *Client) SetBreakpointsRequestWithArgs(file string, lines []int, conditions, hitConditions, logMessages map[int]string) error {
	request := &dap.SetBreakpointsRequest{Request: *c.newRequest("setBreakpoints")}
	request.Arguments = dap.SetBreakpointsArguments{
		Source: dap.Source{
			Name: filepath.Base(file),
			Path: file,
		},
		Breakpoints: make([]dap.SourceBreakpoint, len(lines)),
	}
	for i, l := range lines {
		request.Arguments.Breakpoints[i].Line = l
		if cond, ok := conditions[l]; ok {
			request.Arguments.Breakpoints[i].Condition = cond
		}
		if hitCond, ok := hitConditions[l]; ok {
			request.Arguments.Breakpoints[i].HitCondition = hitCond
		}
		if logMessage, ok := logMessages[l]; ok {
			request.Arguments.Breakpoints[i].LogMessage = logMessage
		}
	}
	return c.send(request)
}

// SetExceptionBreakpointsRequest sends a 'setExceptionBreakpoints' request.
func (c *Client) SetExceptionBreakpointsRequest() {
	request := &dap.SetBreakpointsRequest{Request: *c.newRequest("setExceptionBreakpoints")}
	c.send(request)
}

// ConfigurationDoneRequest sends a 'configurationDone' request.
func (c *Client) ConfigurationDoneRequest() error {
	request := &dap.ConfigurationDoneRequest{Request: *c.newRequest("configurationDone")}
	return c.send(request)
}

// ContinueRequest sends a 'continue' request.
func (c *Client) ContinueRequest(thread int) error {
	request := &dap.ContinueRequest{Request: *c.newRequest("continue")}
	request.Arguments.ThreadId = thread
	return c.send(request)
}

// NextRequest sends a 'next' request.
func (c *Client) NextRequest(thread int) error {
	request := &dap.NextRequest{Request: *c.newRequest("next")}
	request.Arguments.ThreadId = thread
	return c.send(request)
}

// NextInstructionRequest sends a 'next' request with granularity 'instruction'.
func (c *Client) NextInstructionRequest(thread int) {
	request := &dap.NextRequest{Request: *c.newRequest("next")}
	request.Arguments.ThreadId = thread
	request.Arguments.Granularity = "instruction"
	c.send(request)
}

// StepInRequest sends a 'stepIn' request.
func (c *Client) StepInRequest(thread int) {
	request := &dap.StepInRequest{Request: *c.newRequest("stepIn")}
	request.Arguments.ThreadId = thread
	c.send(request)
}

// StepInInstructionRequest sends a 'stepIn' request with granularity 'instruction'.
func (c *Client) StepInInstructionRequest(thread int) {
	request := &dap.StepInRequest{Request: *c.newRequest("stepIn")}
	request.Arguments.ThreadId = thread
	request.Arguments.Granularity = "instruction"
	c.send(request)
}

// StepOutRequest sends a 'stepOut' request.
func (c *Client) StepOutRequest(thread int) {
	request := &dap.StepOutRequest{Request: *c.newRequest("stepOut")}
	request.Arguments.ThreadId = thread
	c.send(request)
}

// StepOutInstructionRequest sends a 'stepOut' request with granularity 'instruction'.
func (c *Client) StepOutInstructionRequest(thread int) {
	request := &dap.StepOutRequest{Request: *c.newRequest("stepOut")}
	request.Arguments.ThreadId = thread
	request.Arguments.Granularity = "instruction"
	c.send(request)
}

// PauseRequest sends a 'pause' request.
func (c *Client) PauseRequest(threadId int) {
	request := &dap.PauseRequest{Request: *c.newRequest("pause")}
	request.Arguments.ThreadId = threadId
	c.send(request)
}

// ThreadsRequest sends a 'threads' request.
func (c *Client) ThreadsRequest() error {
	request := &dap.ThreadsRequest{Request: *c.newRequest("threads")}
	return c.send(request)
}

// StackTraceRequest sends a 'stackTrace' request.
func (c *Client) StackTraceRequest(threadID, startFrame, levels int) {
	request := &dap.StackTraceRequest{Request: *c.newRequest("stackTrace")}
	request.Arguments.ThreadId = threadID
	request.Arguments.StartFrame = startFrame
	request.Arguments.Levels = levels
	c.send(request)
}

// ScopesRequest sends a 'scopes' request.
func (c *Client) ScopesRequest(frameID int) {
	request := &dap.ScopesRequest{Request: *c.newRequest("scopes")}
	request.Arguments.FrameId = frameID
	c.send(request)
}

// VariablesRequest sends a 'variables' request.
func (c *Client) VariablesRequest(variablesReference int) {
	request := &dap.VariablesRequest{Request: *c.newRequest("variables")}
	request.Arguments.VariablesReference = variablesReference
	c.send(request)
}

// IndexedVariablesRequest sends a 'variables' request.
func (c *Client) IndexedVariablesRequest(variablesReference, start, count int) {
	request := &dap.VariablesRequest{Request: *c.newRequest("variables")}
	request.Arguments.VariablesReference = variablesReference
	request.Arguments.Filter = "indexed"
	request.Arguments.Start = start
	request.Arguments.Count = count
	c.send(request)
}

// NamedVariablesRequest sends a 'variables' request.
func (c *Client) NamedVariablesRequest(variablesReference int) {
	request := &dap.VariablesRequest{Request: *c.newRequest("variables")}
	request.Arguments.VariablesReference = variablesReference
	request.Arguments.Filter = "named"
	c.send(request)
}

// TerminateRequest sends a 'terminate' request.
func (c *Client) TerminateRequest() {
	c.send(&dap.TerminateRequest{Request: *c.newRequest("terminate")})
}

// RestartRequest sends a 'restart' request.
func (c *Client) RestartRequest() {
	c.send(&dap.RestartRequest{Request: *c.newRequest("restart")})
}

// SetFunctionBreakpointsRequest sends a 'setFunctionBreakpoints' request.
func (c *Client) SetFunctionBreakpointsRequest(breakpoints []dap.FunctionBreakpoint) {
	c.send(&dap.SetFunctionBreakpointsRequest{
		Request: *c.newRequest("setFunctionBreakpoints"),
		Arguments: dap.SetFunctionBreakpointsArguments{
			Breakpoints: breakpoints,
		},
	})
}

// SetInstructionBreakpointsRequest sends a 'setInstructionBreakpoints' request.
func (c *Client) SetInstructionBreakpointsRequest(breakpoints []dap.InstructionBreakpoint) {
	c.send(&dap.SetInstructionBreakpointsRequest{
		Request: *c.newRequest("setInstructionBreakpoints"),
		Arguments: dap.SetInstructionBreakpointsArguments{
			Breakpoints: breakpoints,
		},
	})
}

// StepBackRequest sends a 'stepBack' request.
func (c *Client) StepBackRequest() {
	c.send(&dap.StepBackRequest{Request: *c.newRequest("stepBack")})
}

// ReverseContinueRequest sends a 'reverseContinue' request.
func (c *Client) ReverseContinueRequest() {
	c.send(&dap.ReverseContinueRequest{Request: *c.newRequest("reverseContinue")})
}

// SetVariableRequest sends a 'setVariable' request.
func (c *Client) SetVariableRequest(variablesRef int, name, value string) {
	request := &dap.SetVariableRequest{Request: *c.newRequest("setVariable")}
	request.Arguments.VariablesReference = variablesRef
	request.Arguments.Name = name
	request.Arguments.Value = value
	c.send(request)
}

// RestartFrameRequest sends a 'restartFrame' request.
func (c *Client) RestartFrameRequest() {
	c.send(&dap.RestartFrameRequest{Request: *c.newRequest("restartFrame")})
}

// GotoRequest sends a 'goto' request.
func (c *Client) GotoRequest() {
	c.send(&dap.GotoRequest{Request: *c.newRequest("goto")})
}

// SetExpressionRequest sends a 'setExpression' request.
func (c *Client) SetExpressionRequest() {
	c.send(&dap.SetExpressionRequest{Request: *c.newRequest("setExpression")})
}

// SourceRequest sends a 'source' request.
func (c *Client) SourceRequest() {
	c.send(&dap.SourceRequest{Request: *c.newRequest("source")})
}

// TerminateThreadsRequest sends a 'terminateThreads' request.
func (c *Client) TerminateThreadsRequest() {
	c.send(&dap.TerminateThreadsRequest{Request: *c.newRequest("terminateThreads")})
}

// EvaluateRequest sends a 'evaluate' request.
func (c *Client) EvaluateRequest(expr string, fid int, context string) {
	request := &dap.EvaluateRequest{Request: *c.newRequest("evaluate")}
	request.Arguments.Expression = expr
	request.Arguments.FrameId = fid
	request.Arguments.Context = context
	c.send(request)
}

// StepInTargetsRequest sends a 'stepInTargets' request.
func (c *Client) StepInTargetsRequest() {
	c.send(&dap.StepInTargetsRequest{Request: *c.newRequest("stepInTargets")})
}

// GotoTargetsRequest sends a 'gotoTargets' request.
func (c *Client) GotoTargetsRequest() {
	c.send(&dap.GotoTargetsRequest{Request: *c.newRequest("gotoTargets")})
}

// CompletionsRequest sends a 'completions' request.
func (c *Client) CompletionsRequest() {
	c.send(&dap.CompletionsRequest{Request: *c.newRequest("completions")})
}

// ExceptionInfoRequest sends a 'exceptionInfo' request.
func (c *Client) ExceptionInfoRequest(threadID int) {
	request := &dap.ExceptionInfoRequest{Request: *c.newRequest("exceptionInfo")}
	request.Arguments.ThreadId = threadID
	c.send(request)
}

// LoadedSourcesRequest sends a 'loadedSources' request.
func (c *Client) LoadedSourcesRequest() {
	c.send(&dap.LoadedSourcesRequest{Request: *c.newRequest("loadedSources")})
}

// DataBreakpointInfoRequest sends a 'dataBreakpointInfo' request.
func (c *Client) DataBreakpointInfoRequest() {
	c.send(&dap.DataBreakpointInfoRequest{Request: *c.newRequest("dataBreakpointInfo")})
}

// SetDataBreakpointsRequest sends a 'setDataBreakpoints' request.
func (c *Client) SetDataBreakpointsRequest() {
	c.send(&dap.SetDataBreakpointsRequest{Request: *c.newRequest("setDataBreakpoints")})
}

// ReadMemoryRequest sends a 'readMemory' request.
func (c *Client) ReadMemoryRequest() {
	c.send(&dap.ReadMemoryRequest{Request: *c.newRequest("readMemory")})
}

// DisassembleRequest sends a 'disassemble' request.
func (c *Client) DisassembleRequest(memoryReference string, instructionOffset, inctructionCount int) {
	c.send(&dap.DisassembleRequest{
		Request: *c.newRequest("disassemble"),
		Arguments: dap.DisassembleArguments{
			MemoryReference:   memoryReference,
			Offset:            0,
			InstructionOffset: instructionOffset,
			InstructionCount:  inctructionCount,
			ResolveSymbols:    false,
		},
	})
}

// CancelRequest sends a 'cancel' request.
func (c *Client) CancelRequest() {
	c.send(&dap.CancelRequest{Request: *c.newRequest("cancel")})
}

// BreakpointLocationsRequest sends a 'breakpointLocations' request.
func (c *Client) BreakpointLocationsRequest() {
	c.send(&dap.BreakpointLocationsRequest{Request: *c.newRequest("breakpointLocations")})
}

// ModulesRequest sends a 'modules' request.
func (c *Client) ModulesRequest() {
	c.send(&dap.ModulesRequest{Request: *c.newRequest("modules")})
}

// UnknownRequest triggers dap.DecodeProtocolMessageFieldError.
func (c *Client) UnknownRequest() {
	request := c.newRequest("unknown")
	c.send(request)
}

// UnknownEvent triggers dap.DecodeProtocolMessageFieldError.
func (c *Client) UnknownEvent() {
	event := &dap.Event{}
	event.Type = "event"
	event.Seq = -1
	event.Event = "unknown"
	c.send(event)
}

// BadRequest triggers an unmarshal error.
func (c *Client) BadRequest() {
	content := []byte("{malformedString}")
	contentLengthHeaderFmt := "Content-Length: %d\r\n\r\n"
	header := fmt.Sprintf(contentLengthHeaderFmt, len(content))
	c.conn.Write([]byte(header))
	c.conn.Write(content)
}

// KnownEvent passes decode checks, but delve has no 'case' to
// handle it. This behaves the same way a new request type
// added to go-dap, but not to delve.
func (c *Client) KnownEvent() {
	event := &dap.Event{}
	event.Type = "event"
	event.Seq = -1
	event.Event = "terminated"
	c.send(event)
}

func (c *Client) newRequest(command string) *dap.Request {
	request := &dap.Request{}
	request.Type = "request"
	request.Command = command
	request.Seq = c.seq
	c.seq++
	return request
}

func (c *Client) ReadInitializedEvent() (*dap.InitializedEvent, error) {
	m, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}

	r, ok := m.(*dap.InitializedEvent)
	if !ok {
		return nil, fmt.Errorf("Read a message but it was not a dap.InitializedEvent:\n %v+", m)
	}
	return r, nil
}

func (c *Client) ReadLaunchResponse() (*dap.LaunchResponse, error) {
	m, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}

	r, ok := m.(*dap.LaunchResponse)
	if !ok {
		return nil, fmt.Errorf("Read a message but it was not a dap.LaunchResponse")
	}
	return r, nil
}

func (c *Client) ReadSetBreakpointsResponse() (*dap.SetBreakpointsResponse, error) {
	m, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}

	r, ok := m.(*dap.SetBreakpointsResponse)
	if !ok {
		return nil, fmt.Errorf("Read a message but it was not a dap.SetBreakpointsResponse")
	}
	return r, nil
}

func (c *Client) ReadStoppedEvent() (*dap.StoppedEvent, error) {
	m, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}

	r, ok := m.(*dap.StoppedEvent)
	if !ok {
		return nil, fmt.Errorf("Read a message but it was not a dap.StoppedEvent")
	}
	return r, nil
}

func (c *Client) ReadConfigurationDoneResponse() (*dap.ConfigurationDoneResponse, error) {
	m, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}

	r, ok := m.(*dap.ConfigurationDoneResponse)
	if !ok {
		return nil, fmt.Errorf("Read a message but it was not a dap.ConfigurationDoneResponse")
	}
	return r, nil
}

func (c *Client) ReadContinueResponse() (*dap.ContinueResponse, error) {
	m, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}

	r, ok := m.(*dap.ContinueResponse)
	if !ok {
		return nil, fmt.Errorf("Read a message but it was not a dap.ContinueResponse")
	}
	return r, nil
}

func (c *Client) ReadThreadsResponse() (*dap.ThreadsResponse, error) {
	m, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}

	r, ok := m.(*dap.ThreadsResponse)
	if !ok {
		return nil, fmt.Errorf("Read a message but it was not a dap.ThreadsResponse")
	}
	return r, nil
}

func (c *Client) ReadTerminatedEvent() (*dap.TerminatedEvent, error) {
	m, err := c.ReadMessage()
	if err != nil {
		return nil, err
	}

	r, ok := m.(*dap.TerminatedEvent)
	if !ok {
		return nil, fmt.Errorf("Read a message but it was not a dap.TerminatedEvent")
	}
	return r, nil
}
