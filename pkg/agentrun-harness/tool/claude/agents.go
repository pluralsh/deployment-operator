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
	"tools": ["Read", "Write", "Edit", "MultiEdit", "Bash", "Grep", "Glob", "WebFetch", "mcp__plural__agentPullRequest", "mcp__plural__createBranch", "mcp__plural__fetchAgentRunTodos", "mcp__plural__updateAgentRunTodos", "mcp__plural__updateAgentRunAnalysis", "docker"]
  }
}`
