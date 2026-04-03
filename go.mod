module github.com/benaskins/axon-scan

go 1.26.1

replace (
	github.com/benaskins/axon-loop => /Users/benaskins/dev/lamina/axon-loop
	github.com/benaskins/axon-talk => /Users/benaskins/dev/lamina/axon-talk
	github.com/benaskins/axon-tool => /Users/benaskins/dev/lamina/axon-tool
)

require (
	github.com/benaskins/axon-loop v0.7.4
	github.com/benaskins/axon-tool v0.3.0
)

require (
	github.com/benaskins/axon-talk v0.5.1 // indirect
	github.com/benaskins/axon-tape v0.1.1 // indirect
)
