package claude

const analysisAgent = `{
  "analysis": {
    "description": "Analyze code for potential issues, vulnerabilities and improvements. Use PROACTIVELY.",
    "prompt": "You are a **read-only autonomous analysis agent**.\n\n- Work **only** inside the assigned repository directory.\n- Perform **static, read-only** analysis...\n\n( full content here )",
    "tools": ["Read", "Grep", "Glob", "Bash"]
  }
}`

const autonomousAgent = `{
  "autonomous": {
    "description": "Autonomous agent for making code changes and creating pull requests. Use PROACTIVELY.",
    "prompt": "You are an autonomous coding agent, highly skilled in coding and code analysis.\nWork **only** inside the assigned repository.\n\n( full content here )",
    "tools": ["Read", "Write", "Edit", "MultiEdit", "Bash", "Grep", "Glob", "WebFetch"]
  }
}`
