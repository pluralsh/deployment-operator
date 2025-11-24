package tool

import (
	"strings"

	"github.com/pluralsh/console/go/client"
	"github.com/samber/lo"
)

var (
	cachedAgentRun *client.AgentRunFragment
)

func GetCachedTodos() []*client.AgentTodoFragment {
	if cachedAgentRun == nil {
		return nil
	}

	return cachedAgentRun.Todos
}

func UpdateCachedTodos(todos []*client.AgentTodoFragment) {
	if cachedAgentRun == nil {
		cachedAgentRun = &client.AgentRunFragment{}
	}

	cachedAgentRun.Todos = todos
}

func MarkCachedTodoItem(title string, done bool) bool {
	if cachedAgentRun == nil || cachedAgentRun.Todos == nil || done == false {
		return false
	}

	for i, todo := range cachedAgentRun.Todos {
		if strings.Contains(todo.Title, title) {
			cachedAgentRun.Todos[i].Done = lo.ToPtr(true)
			return true
		}
	}

	return false
}
