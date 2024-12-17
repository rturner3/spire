//go:build windows

package clitest

var (
	AddrArg         = "-namedPipeName"
	AddrError       = "Error: connection error: desc = \"transport: error while dialing: open \\\\\\\\.\\\\pipe\\\\does-not-exist: The system cannot find the file specified.\"\n"
	AddrOutputUsage = `
  -namedPipeName string
    	Pipe name of the SPIRE Server API named pipe (default "\\spire-server\\private\\api")
  -output value
    	Desired output format (pretty, json); default: pretty.
`
	AddrValue = "\\does-not-exist"
)