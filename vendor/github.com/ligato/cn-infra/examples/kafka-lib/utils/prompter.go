// Copyright (c) 2017 Cisco and/or its affiliates.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at:
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"fmt"

	"github.com/Songmu/prompter"
)

// Message contains the message elements to prompt for.
type Message struct {
	Topic    string
	Text     string
	Key      string
	Metadata string
}

// Command to prompt for.
type Command struct {
	Cmd string
	Message
}

// GetCommand allows user to select a command and reads the input arguments.
func GetCommand() *Command {
	var command *Command
loop:
	for {
		cmd := prompter.Choose("\nEnter command", []string{"quit", "message"}, "message")
		command = new(Command)
		switch cmd {
		case "quit":
			command.Cmd = "quit"
			fin := prompter.YN("Quit?", true)
			if fin {
				break loop
			}
		case "message":
			command.Cmd = "message"
			var (
				topic string
				text  string
			)
			for {
				topic = prompter.Prompt("Enter topic (REQUIRED)", "")
				if topic == "" {
					fmt.Println("topic cannot be blank")
				} else {
					break
				}
			}
			for {
				text = prompter.Prompt("Enter message (REQUIRED)", "")
				if text == "" {
					fmt.Println("message cannot be blank")
				} else {
					break
				}
			}
			command.Topic = topic
			command.Text = text
			key := prompter.Prompt("Enter key", "")
			command.Key = key
			metadata := prompter.Prompt("Enter metadata", "")
			command.Metadata = metadata
			send := prompter.YN("Send Message?", true)
			if send {
				break loop
			}
		}
	}
	return command
}
