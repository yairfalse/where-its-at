Agent Instructions (BRUTAL)

Build Requirements MUST FORMAT: make fmt before any commit MUST COMPILE: go build ./... must pass MUST TEST: go test ./... must pass NO STUBS: No "TODO", "not implemented", empty functions SHOW PROOF: Paste build/test output or FAIL NO PUSH TO MAIN: We work with branches and PR.

Quality Standards 80% test coverage minimum you must create unit testing to your code full coverge. No map[string]interface{} in public APIs No interface{} abuse Proper error handling with context NO Stubs, no shortcuts YOU work on a dedicated branch Code must be readble

Verification (MANDATORY)

You MUST show this output:
make fmt # Format code first gofmt -l . | grep -v vendor | wc -l # MUST return 0 go build ./... go test ./... go mod verify

🔧 Current Priorities Getting the whole thing to work Success Metrics All collectors building independently ✅ Semantic correlation working ✅ CI/CD enforcement active ✅ Revenue features ready

Core Mission Root Cause Analysis: Every correlation must identify WHY, not just WHAT happened Best Practices: No shortcuts, no stubs, no endless loops of "fixing/thinking/asking" - deliver working solutions

🚫 Failure Conditions Instant Task Reassignment If Code not formatted (gofmt failures) Build errors Test failures Architectural violations Missing verification output Stub functions or TODOs work on dedicated branchs, PR only your code!

No Excuses For "Forgot to format" - Always run make fmt "Complex existing code" - Use what exists "Need to refactor first" - Follow requirements "Just one small TODO" - Zero tolerance "Can't find interfaces" - Ask for help "fast solutions"- ask for advice

📋 Task Template Every task must include:

Verification Results
Code Formatting:
$ make fmt
[PASTE OUTPUT - should show "Code formatted successfully" or similar]

$ gofmt -l . | grep -v vendor | wc -l
0
[MUST be 0 - if not 0, code is not properly formatted]
Build Test
$ go build ./...
[PASTE OUTPUT]
Unit Tests
$ go test ./...
[PASTE OUTPUT]
Files Created
file1.go (X lines)
file2.go (Y lines) Total: Z lines

Architecture Compliance
✅ Code properly formatted ✅ Follows 5-level hierarchy ✅ Independent go.mod ✅ No architectural violations ✅ Proper imports only


## 🎯 Bottom Line

**Format code. Build working code. Prove it works. Follow architecture. No shortcuts.**

Deliver or get reassigned.
