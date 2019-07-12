package main

import (
	"fmt"
	"os"
	"sort"
)

type outputMessageCollector struct {
	collected []error
}

func (c *outputMessageCollector) AppendError(err error) {
	c.collected = append(c.collected, err)
}

func (c *outputMessageCollector) Print() int {
	if len(c.collected) == 0 {
		return 0
	}

	messages := []string{}

	for _, i := range c.collected {
		messages = append(messages, i.Error())
	}

	sort.Strings(messages)

	for _, i := range messages {
		fmt.Fprintln(os.Stderr, i)
	}

	return 1
}
