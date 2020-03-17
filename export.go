package main

import (
	"fmt"
	"os"
	"strconv"
	"text/template"
)

func exportLaunchd(app string, entries []procfileEntry, formations map[string]formationEntry, location string, defaultPort int, vars map[string]interface{}) bool {
	l, err := loadTemplate("launchd", "templates/launchd/launchd.plist.tmpl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return false
	}

	if _, err := os.Stat(location); os.IsNotExist(err) {
		os.MkdirAll(location, os.ModePerm)
	}

	for i, entry := range entries {
		count := processCount(entry, formations)

		num := 1
		for num <= count {
			processName := fmt.Sprintf("%s-%d", entry.Name, num)
			fmt.Println("writing:", app+"-"+processName+".plist")

			port := portFor(i, num, defaultPort)
			config := templateVars(app, entry, processName, num, port, vars)
			if !writeOutput(l, location+"/"+app+"-"+processName+".plist", config) {
				return false
			}

			num += 1
		}
	}

	return true
}

func exportRunit(app string, entries []procfileEntry, formations map[string]formationEntry, location string, defaultPort int, vars map[string]interface{}) bool {
	r, err := loadTemplate("run", "templates/runit/run.tmpl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return false
	}
	l, err := loadTemplate("log", "templates/runit/log/run.tmpl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return false
	}

	if _, err := os.Stat(location); os.IsNotExist(err) {
		os.MkdirAll(location, os.ModePerm)
	}

	for i, entry := range entries {
		count := processCount(entry, formations)

		num := 1
		for num <= count {
			processDirectory := fmt.Sprintf("%s-%s-%d", app, entry.Name, num)
			folderPath := location + "/" + processDirectory
			processName := fmt.Sprintf("%s-%d", entry.Name, num)

			fmt.Println("creating:", app+"-"+processName)
			os.MkdirAll(folderPath, os.ModePerm)

			fmt.Println("creating:", app+"-"+processName+"/env")
			os.MkdirAll(folderPath+"/env", os.ModePerm)

			fmt.Println("creating:", app+"-"+processName+"/log")
			os.MkdirAll(folderPath+"/log", os.ModePerm)

			port := portFor(i, num, defaultPort)
			config := templateVars(app, entry, processName, num, port, vars)

			fmt.Println("writing:", app+"-"+processName+"/run")
			if !writeOutput(r, folderPath+"/run", config) {
				return false
			}

			env, ok := config["env"].(map[string]string)
			if !ok {
				fmt.Fprintf(os.Stderr, "invalid env map\n")
				return false
			}

			env["PORT"] = strconv.Itoa(port)
			env["PS"] = app + "-" + processName

			for key, value := range env {
				fmt.Println("writing:", app+"-"+processName+"/env/"+key)
				f, err := os.Create(folderPath + "/env/" + key)
				if err != nil {
					fmt.Fprintf(os.Stderr, "error creating file: %s\n", err)
					return false
				}
				defer f.Close()

				if _, err = f.WriteString(value); err != nil {
					fmt.Fprintf(os.Stderr, "error writing output: %s\n", err)
					return false
				}

				if err = f.Sync(); err != nil {
					fmt.Fprintf(os.Stderr, "error syncing output: %s\n", err)
					return false
				}
			}

			fmt.Println("writing:", app+"-"+processName+"/log/run")
			if !writeOutput(l, folderPath+"/log/run", config) {
				return false
			}

			num += 1
		}
	}

	return true
}

func exportSystemd(app string, entries []procfileEntry, formations map[string]formationEntry, location string, defaultPort int, vars map[string]interface{}) bool {
	t, err := loadTemplate("target", "templates/systemd/default/control.target.tmpl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return false
	}

	s, err := loadTemplate("service", "templates/systemd/default/program.service.tmpl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return false
	}

	if _, err := os.Stat(location); os.IsNotExist(err) {
		os.MkdirAll(location, os.ModePerm)
	}

	processes := []string{}
	for i, entry := range entries {
		count := processCount(entry, formations)

		num := 1
		for num <= count {
			processName := fmt.Sprintf("%s-%d", entry.Name, num)
			fileName := fmt.Sprintf("%s.%d", entry.Name, num)
			processes = append(processes, fmt.Sprintf(app+"-%s.service", fileName))
			fmt.Println("writing:", app+"-"+fileName+".service")

			port := portFor(i, num, defaultPort)
			config := templateVars(app, entry, processName, num, port, vars)
			if !writeOutput(s, location+"/"+app+"-"+fileName+".service", config) {
				return false
			}

			num += 1
		}
	}

	config := vars
	config["processes"] = processes
	fmt.Println("writing:", app+".target")
	return writeOutput(t, location+"/"+app+".target", config)
}

func exportSystemdUser(app string, entries []procfileEntry, formations map[string]formationEntry, location string, defaultPort int, vars map[string]interface{}) bool {
	s, err := loadTemplate("service", "templates/systemd-user/default/program.service.tmpl")
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s\n", err)
		return false
	}

	if _, err := os.Stat(location); os.IsNotExist(err) {
		os.MkdirAll(location, os.ModePerm)
	}

	processes := []string{}
	for i, entry := range entries {
		count := processCount(entry, formations)

		num := 1
		for num <= count {
			processName := fmt.Sprintf("%s-%d", entry.Name, num)
			processes = append(processes, fmt.Sprintf("%s.service", processName))
			fmt.Println("writing:", app+"-"+processName+".service")

			port := portFor(i, num, defaultPort)
			config := templateVars(app, entry, processName, num, port, vars)
			if !writeOutput(s, location+"/"+app+"-"+processName+".service", config) {
				return false
			}

			num += 1
		}
	}

	return true
}

func processCount(entry procfileEntry, formations map[string]formationEntry) int {
	count := 0
	if f, ok := formations["all"]; ok {
		count = f.Count
	}
	if f, ok := formations[entry.Name]; ok {
		count = f.Count
	}
	return count
}

func portFor(processIndex int, instance int, base int) int {
	return 5000 + (processIndex * 100) + (instance - 1)
}

func templateVars(app string, entry procfileEntry, processName string, num int, port int, vars map[string]interface{}) map[string]interface{} {
	config := vars
	config["command"] = entry.Command
	config["command_args"] = entry.commandArgs()
	config["num"] = num
	config["port"] = port
	config["process_name"] = processName
	config["process_type"] = entry.Name
	config["ps"] = app + "-" + entry.Name + "." + strconv.Itoa(num)
	if config["description"] == "" {
		config["description"] = fmt.Sprintf("%s process for %s", processName, app)
	}

	return config
}

func writeOutput(t *template.Template, outputPath string, variables map[string]interface{}) bool {
	f, err := os.Create(outputPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error creating file: %s\n", err)
		return false
	}
	defer f.Close()

	if err = t.Execute(f, variables); err != nil {
		fmt.Fprintf(os.Stderr, "error writing output: %s\n", err)
		return false
	}

	if err := os.Chmod(outputPath, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "error setting mode: %s\n", err)
		return false
	}

	return true
}

func loadTemplate(name string, filename string) (*template.Template, error) {
	asset, err := Asset(filename)
	if err != nil {
		return nil, err
	}

	t, err := template.New(name).Parse(string(asset))
	if err != nil {
		return nil, fmt.Errorf("error parsing template: %s", err)
	}

	return t, nil
}
