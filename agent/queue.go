package agent

import (
	recipe "github.com/coreos/etcd/contrib/recipes"
)

func NewQueue() {
	recipe.NewPriorityQueue(nil, "")
}
