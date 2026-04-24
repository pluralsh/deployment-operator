package claude

const analysisAgent = `{
  "analysis": {
    "description": "Analyze code for potential issues, vulnerabilities and improvements. Use PROACTIVELY.",
    "prompt": "You are a read-only autonomous analysis agent.",
    "tools": ["Read", "Grep", "Glob", "Bash", "mcp__plural__updateAgentRunAnalysis"]
  }
}`

const autonomousAgent = `{
  "autonomous": {
    "description": "Autonomous agent for making code changes and creating pull requests. Use PROACTIVELY.",
    "prompt": "You are an autonomous coding agent, highly skilled in coding and code analysis.",
	"tools": ["Read", "Write", "Edit", "MultiEdit", "Bash", "Grep", "Glob", "WebFetch", "mcp__plural__agentPullRequest", "mcp__plural__createBranch", "mcp__plural__fetchAgentRunTodos", "mcp__plural__updateAgentRunTodos"]
  }
}`

const babysitAgent = `{
  "autonomous": {
    "description": "Responds to pull request reviewer comments and fixes CI failures on an existing PR branch. Use PROACTIVELY.",
    "prompt": "You are an autonomous coding agent in babysit mode. Your PR is already open. Address reviewer comments and fix CI failures, then commit to the existing PR branch.",
	"tools": ["Read", "Write", "Edit", "MultiEdit", "Bash", "Grep", "Glob", "WebFetch", "mcp__plural__createBranch", "mcp__plural__fetchAgentRunTodos", "mcp__plural__updateAgentRunTodos"]
  }
}`
