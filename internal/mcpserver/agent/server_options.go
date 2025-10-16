package agent

// WithTools enables the MCP server to support tools
func WithTools() Option {
	return func(s *Server) {
		s.toolsEnabled = true
	}
}

// WithTool registers a tool with the MCP server
func WithTool(tool Tool) Option {
	return func(s *Server) {
		s.tools = append(s.tools, tool)
	}
}

// WithVersion sets the MCP server version
func WithVersion(version string) Option {
	return func(s *Server) {
		s.version = version
	}
}
