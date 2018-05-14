package tasks

import "log"

func ModuleManager(killFlag *bool, moduleCommands chan moduleCommand) {
	for !*killFlag {
		select {
		case command := <-moduleCommands:
			log.Printf("Recieved command for %s: %s", command.ModuleName, command.Command)
		}
	}
}