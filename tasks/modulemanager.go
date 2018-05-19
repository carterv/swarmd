package tasks

import (
	"log"
	"path/filepath"
	"swarmd/util"
	"os"
	"strings"
	"fmt"
	"os/exec"
	"runtime"
)

func GetModulePath() string {
	modulePath := filepath.Join(util.GetBasePath(), "modules/")

	// Make the module directory if it doesn't exist
	os.MkdirAll(modulePath, 0700)

	return modulePath
}

func UnpackModule(archive string) {
	moduleName := strings.TrimSuffix(filepath.Base(archive), filepath.Ext(archive))
	modulePath := filepath.Join(GetModulePath(), moduleName)

	_, err := util.Unzip(archive, modulePath)
	if err != nil {
		log.Print(archive)
		log.Print(err)
	}
}

func ModuleManager(killFlag *bool, moduleCommands chan moduleCommand) {
	for !*killFlag {
		select {
		case command := <-moduleCommands:
			log.Printf("Received command for %s: %s", command.ModuleName, command.Command)
			go handleCommand(command)
		}
	}
}

func moduleDataExists(moduleName string) bool {
	_, err := os.Stat(filepath.Join(GetSharePath(), fmt.Sprintf("%s.swm", moduleName)))
	return err == nil
}

func moduleInstalled(moduleName string) bool {
	_, err := os.Stat(filepath.Join(GetModulePath(), moduleName))
	return err == nil
}

func handleCommand(cmd moduleCommand) {
	moduleDir := filepath.Join(GetModulePath(), cmd.ModuleName)
	switch cmd.Command {
	case "install":
		if !moduleDataExists(cmd.ModuleName) {
			break
		}
		UnpackModule(filepath.Join(GetSharePath(), fmt.Sprintf("%s.swm", cmd.ModuleName)))
		installScript := filepath.Join(moduleDir, "install")
		runScript(installScript, moduleDir)
	case "uninstall":
		if !moduleInstalled(cmd.ModuleName) {
			break
		}
		uninstallScript := filepath.Join(moduleDir, "uninstall")
		runScript(uninstallScript, moduleDir)
		os.RemoveAll(moduleDir)
	case "start":
		if !moduleInstalled(cmd.ModuleName) {
			break
		}
		startScript := filepath.Join(moduleDir, "start")
		runScript(startScript, moduleDir)
	case "stop":
		if !moduleInstalled(cmd.ModuleName) {
			break
		}
		stopScript := filepath.Join(moduleDir, "stop")
		runScript(stopScript, moduleDir)
	default:
		log.Printf("Recieved unknown command: %s", cmd.Command)
	}
}

func runScript(script string, workingDir string) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("powershell", strings.Join([]string{script, "ps1"}, "."))
	} else {
		cmd = exec.Command("bash", strings.Join([]string{script, "sh"}, "."))
	}
	cmd.Dir = workingDir
	port, present := os.LookupEnv("SWARMD_LOCAL_PORT")
	if !present {
		port = "51234"
	}
	cmd.Env = append(os.Environ(), fmt.Sprintf("SWARMD_LOCAL_PORT=%s", port))
	output, err := cmd.Output()
	if err != nil {
		log.Print(err)
	}
	log.Print(string(output))
}
