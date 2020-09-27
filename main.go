package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml"
)

func main() {
	configFile := "extensions.toml"
	extensionsDir := "extensions"
	workspaceDir := "workspaces"

	args := os.Args[1:]

	if contains(args, "--help") || contains(args, "-h") {
		fmt.Println(`sparta

Description:
  Contextual vscode extension management

Commands:
  clone
	 Clones your current extensions to a local folder

  init
    Initiates an 'extensions.toml' folder that contains all extensions for tagging

  generate
    Generates all extension folders to be used by vscode

  clear
    Removes all downloaded extensions from their workspaces

  check
    Prints all extensions mismatches between default globally installed and ones defined in extensions.toml

  launch [workspace]
    Launches a particular workspace in vscode`)
		os.Exit(0)
	}

	ensureLength(args, 1, "Must specify command. Exiting")

	command := args[0]
	if command == "clone" {

		currentExtensions := getVscodeExtensions()
		for _, extension := range currentExtensions {
			if extension == "" {
				continue
			}

			fmt.Printf("EXTENSION: %s\n", extension)

			// install extensions
			cmd := exec.Command("code", "--extensions-dir", extensionsDir, "--install-extension", extension, "--force")
			cmd.Stderr = os.Stderr
			stdout, err := cmd.Output()
			p(err)

			fmt.Println(string(stdout))
		}

		// now, we have to rename all the files to remove the version number
		dirs, err := ioutil.ReadDir(extensionsDir)
		p(err)

		for _, dir := range dirs {
			// if extension doesn't have version, it has already
			// been renamed
			if !extensionHasVersion(dir.Name()) {
				continue
			}

			// take off the version number
			parts := strings.Split(dir.Name(), "-")
			newName := strings.Join(parts[:len(parts)-1], "-")
			newName = strings.ToLower(newName)

			old := filepath.Join(extensionsDir, dir.Name())
			new := filepath.Join(extensionsDir, newName)

			err := os.Rename(old, new)
			p(err)

			fmt.Printf("Renaming file: '%s'\n", new)
		}

	} else if command == "generate" {
		config := readConfig()

		err := os.RemoveAll(workspaceDir)
		p(err)

		err = os.MkdirAll(workspaceDir, 0755)
		p(err)

		// generate workspaces
		for _, workspace := range config.Workspaces {
			fmt.Printf("WORKSPACE: %s\n", workspace.Name)
			err := os.MkdirAll(filepath.Join(workspaceDir, workspace.Name), 0755)
			if err != nil && !os.IsExist(err) {
				panic(err)
			}

			for _, extension := range config.Extensions {
				fmt.Printf("EXTENSION: %s\n", extension.Name)

				for _, tag := range extension.Tags {
					// if any tag in current extension is used in the workspace
					if contains(workspace.Use, tag) {
						src := filepath.Join("../..", extensionsDir, strings.ToLower(extension.Name))
						dest := filepath.Join(workspaceDir, workspace.Name, extension.Name)

						err := os.Symlink(src, dest)
						if err != nil && !os.IsExist(err) {
							panic(err)
						}

						// go to next extension of a workspace
						continue
					}
				}
			}
		}
	} else if command == "launch" {
		ensureLength(args, 2, "Must pass in a workspace name")
		workspaceName := args[1]
		extensionsDir := filepath.Join(workspaceDir, workspaceName)

		cmd := exec.Command("code", "--extensions-dir", extensionsDir, ".")
		cmd.Stderr = os.Stderr
		stdout, err := cmd.Output()
		p(err)

		fmt.Println(stdout)
	} else if command == "clear" {
		err := os.RemoveAll(workspaceDir)
		p(err)
	} else if command == "init" {
		// create config
		var config Config
		config.Version = "1"

		extensions := getVscodeExtensions()
		for _, extension := range extensions {
			if extension == "" {
				continue
			}

			config.Extensions = append(config.Extensions, Extension{
				Name: extension,
			})
		}

		configRaw, err := toml.Marshal(config)
		p(err)

		// write config file only if it doesn't already exist
		file, err := os.OpenFile(configFile, os.O_CREATE|os.O_EXCL|os.O_RDWR, 0644)
		defer func() {
			file.Close()
			p(err)
		}()

		if err != nil {
			if os.IsExist(err) {
				fmt.Printf("%s already exists. Remove it before continuing. Exiting\n", configFile)
				os.Exit(1)
				return
			}
			panic(err)
		}

		_, err = file.Write(configRaw)
		p(err)
	} else if command == "check" {

		extensions := getVscodeExtensions()
		config := readConfig()

		fmt.Println("Extensions that are installed globally, but could not be found local")
		for _, globalExtension := range extensions {
			isHere := false
			for _, spartaExtension := range config.Extensions {
				if globalExtension == spartaExtension.Name {
					isHere = true
					continue
				}
			}

			if !isHere {
				fmt.Printf("NOT LOCAL: %s\n", globalExtension)
			}
		}

		fmt.Println()
		fmt.Println("Extensions that are installed locally, but not globally")
		for _, spartaExtension := range config.Extensions {
			isGlobal := false

			for _, globalExtension := range extensions {

				if spartaExtension.Name == globalExtension {

					isGlobal = true
					continue
				}
			}

			if !isGlobal {
				fmt.Printf("NOT GLOBAL: %s\n", spartaExtension.Name)
			}
		}

		fmt.Println()
		fmt.Println("Extensions that don't have any tags")
		for _, extension := range config.Extensions {
			if len(extension.Tags) == 0 {
				fmt.Printf("NO TAGS: %s\n", extension.Name)
			}
		}

		fmt.Println()
		fmt.Println("Extensions tags that aren't in a group")
		for _, extension := range config.Extensions {
			for _, tag := range extension.Tags {
				inGroup := false
			g:
				for _, group := range config.Workspaces {
					for _, usedTag := range group.Use {
						if usedTag == tag {
							inGroup = true
							continue g
						}
					}

				}
				if !inGroup {
					fmt.Printf("TAG NOT USED: %s\n", tag)
				}
			}
		}

	} else {
		log.Fatalln("Unknown Command. Exiting")
	}

}
